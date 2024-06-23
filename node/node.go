package node

import (
	"context"
	"errors"
	"log"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/flynn/noise"
	"golang.org/x/net/ipv4"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	conn "github.com/caldog20/overlay/node/conn"
	tun "github.com/caldog20/overlay/node/tun"
	"github.com/caldog20/overlay/pkg/header"
	controllerv1 "github.com/caldog20/overlay/proto/gen/controller/v1"
)

type Node struct {
	conn *conn.Conn // This will change to multiple conns in future
	tun  tun.Tun
	id   uint32
	ip   netip.Prefix

	prefOutboundIP     netip.Addr
	discoveredEndpoint netip.AddrPort

	maps struct {
		l  sync.RWMutex
		id map[uint32]*Peer     // for RX
		ip map[netip.Addr]*Peer // for TX
	}

	noise struct {
		l       sync.RWMutex
		keyPair noise.DHKey
	}

	running atomic.Bool

	controller controllerv1.ControllerServiceClient
	// Temp
	port                        uint16
	controllerAddr              string
	controllerDiscoveryEndpoint *net.UDPAddr
}

func NewNode(port uint16, controller string) (*Node, error) {
	node := new(Node)
	node.maps.id = make(map[uint32]*Peer)
	node.maps.ip = make(map[netip.Addr]*Peer)

	// Try to load key from disk
	keypair, err := LoadKeyFromDisk()
	if err != nil {
		keypair, err = GenerateNewKeypair()
		if err != nil {
			log.Fatal(err)
		}
	}

	node.noise.keyPair = keypair

	if port > 65535 {
		return nil, errors.New("invalid udp port")
	}

	node.conn, err = conn.NewConn(port)
	if err != nil {
		return nil, err
	}

	// _, p, err := net.SplitHostPort(node.conn.LocalAddr().String())
	// if err != nil {
	// 	return nil, err
	// }

	// finalPort, err := strconv.ParseUint(p, 10, 16)
	// if err != nil {
	// 	return nil, err
	// }

	// node.port = uint16(finalPort)

	node.tun, err = tun.NewTun()
	if err != nil {
		return nil, err
	}

	// TODO Fix this/move when fixing login/register flow
	ctx, _ := context.WithTimeout(context.Background(), time.Second*10)
	gconn, err := grpc.DialContext(
		ctx,
		controller,
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatal("error connecting to controller grpc: ", err)
	}

	node.controller = controllerv1.NewControllerServiceClient(gconn)

	node.controllerAddr = controller

	return node, nil
}

func (node *Node) loginOrRegister() error {
	if err := node.Login(); err != nil {
		s, _ := status.FromError(err)
		if s.Code() == codes.NotFound {
			err = node.Register()
			if err != nil {
				return err
			} else {
				return node.Login()
			}
		} else {
			return err
		}
	}

	return nil
}

func (node *Node) Run(ctx context.Context) error {
	// Register with controller
	err := node.loginOrRegister()
	if err != nil {
		return err
	}

	// Configure tunnel ip/routes
	err = node.tun.ConfigureIPAddress(node.ip)
	if err != nil {
		return err
	}

	// Initially set endpoints
	err = node.SendDiscoveryRequest()
	if err != nil {
		return err
	}

	go node.ReadUDPPackets(node.OnUDPPacket, 0)
	go node.ReadTunPackets(node.OnTunnelPacket)

	node.StartUpdateStream(ctx)
	// TODO
	<-ctx.Done()

	node.conn.Close()
	node.tun.Close()

	//for _, peer := range node.maps.id {
	//	if peer.running.Load() {
	//		peer.cancel()
	//	}
	//}
	return nil
}

func (node *Node) lookupPeer(id uint32) (*Peer, bool) {
	node.maps.l.RLock()
	peer, found := node.maps.id[id]
	node.maps.l.RUnlock()

	if !found {
		return nil, false
	}

	// TODO temporary sanity check if peer is somehow nil
	if peer == nil {
		panic("Peer is nil")
	}

	return peer, true
}

func (node *Node) OnUDPPacket(buffer *InboundBuffer, index int) {
	err := buffer.header.Parse(buffer.in)
	if err != nil {
		log.Println(err)
		PutInboundBuffer(buffer)
		return
	}

	// Lookup Peer based on index
	sender := buffer.header.SenderIndex

	// Peer found, check message type and handle accordingly
	switch buffer.header.Type {
	// Remote peer sent handshake message
	case header.Handshake:
		peer, found := node.lookupPeer(sender)
		if !found {
			PutInboundBuffer(buffer)
			log.Printf("[inbound] peer with index %d not found", sender)
			return
		}
		buffer.peer = peer

		// Callee responsible to returning buffer to pool
		if peer.running.Load() {
			peer.handshakes <- buffer
		}
		return
	// Remote peer sent encrypted data
	case header.Data:
		// Callee responsible to returning buffer to pool
		peer, found := node.lookupPeer(sender)
		if !found {
			PutInboundBuffer(buffer)
			log.Printf("[inbound] peer with index %d not found", sender)
			return
		}
		buffer.peer = peer

		peer.InboundPacket(buffer)
		return
	// Remote peer sent punch packet
	case header.Punch:
		log.Printf("[inbound] received punch packet from peer %d", sender)
		PutInboundBuffer(buffer)
		return
	case header.Discovery:
		// Logic to process stun/discovery responses
		node.HandleDiscoveryResponse(buffer)
		PutInboundBuffer(buffer)
		return
	default:
		log.Printf("[inbound] unmatched header: %s", buffer.header.String())
		PutInboundBuffer(buffer)
		return
	}
}

func (node *Node) OnTunnelPacket(buffer *OutboundBuffer) {
	ipHeader, err := ipv4.ParseHeader(buffer.packet)
	if err != nil {
		log.Println("[outbound] failed to parse ipv4 header")
		PutOutboundBuffer(buffer)
		return
	}

	// TODO Move this
	if ipHeader.Dst.Equal(node.ip.Addr().AsSlice()) {
		// destination is local tunnel, drop
		PutOutboundBuffer(buffer)
		return
	}

	// TODO Firewall implementation
	// Check for broadcasting and block
	dst, _ := netip.AddrFromSlice(ipHeader.Dst.To4())
	if !node.ip.Masked().Contains(dst) {
		// destination is not in network, drop
		PutOutboundBuffer(buffer)
		return
	}

	// Lookup peer
	node.maps.l.RLock()
	peer, found := node.maps.ip[dst]
	node.maps.l.RUnlock()
	if !found {
		// peer not found, drop
		log.Printf("[outbound] peer with ip %s not found", dst.String())
		PutOutboundBuffer(buffer)
		//node.UpdateNodes()
		return
	}

	peer.OutboundPacket(buffer)

	return
}
