package node

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/netip"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/caldog20/machineid"
	"github.com/caldog20/zeronet/node/conn"
	"github.com/caldog20/zeronet/node/tun"
	"github.com/caldog20/zeronet/pkg/header"
	nodev1 "github.com/caldog20/zeronet/proto/gen/node/v1"
	"github.com/flynn/noise"
	"github.com/pion/ice/v3"
	"github.com/pion/stun/v2"
	"golang.org/x/net/ipv4"
)

// TODO: Verify need for mutex for node properties like ip, prefix, id, etc
// TODO: Handle logged in state and when to refresh
// TODO: Handle logged in state after running 'down' command
type Node struct {
	udpMux *ice.UniversalUDPMuxDefault
	conn   *conn.Conn // This will change to multiple conns in future
	tun    tun.Tun
	id     uint32
	ip     netip.Prefix

	// TODO Start using mutex for node fields
	lock sync.RWMutex
	//discoveredEndpoint netip.AddrPort
	discoveredEndpoint string

	maps struct {
		l  sync.RWMutex
		id map[uint32]*Peer     // for RX
		ip map[netip.Addr]*Peer // for TX
	}

	noise struct {
		l       sync.RWMutex
		keyPair noise.DHKey
	}

	// TODO: Verify this bool
	running    atomic.Bool
	grpcClient *ControllerClient
	// Temp
	port                        uint16
	controllerDiscoveryEndpoint *net.UDPAddr

	runCtx    context.Context
	runCancel context.CancelFunc
	nodev1.UnimplementedNodeServiceServer
	machineID string
	hostname  string
	loggedIn  atomic.Bool
	stunUrls  []*stun.URI
}

func NewNode(controller string, port uint16) (*Node, error) {
	node := new(Node)
	node.maps.id = make(map[uint32]*Peer)
	node.maps.ip = make(map[netip.Addr]*Peer)

	// TODO: For now, we generate a new key on startup every time
	// TODO: Key rotation periodically
	// Try to load key from disk
	//keypair, err := LoadKeyFromDisk()
	//if err != nil {
	//	keypair, err = GenerateNewKeypair()
	//	if err != nil {
	//		log.Println("error storing keypair to disk")
	//	}
	//}
	keypair, err := GenerateNewKeypair()
	if err != nil {
		return nil, errors.New("could not generate keypair: " + err.Error())
	}

	node.noise.keyPair = keypair
	node.port = port

	node.machineID, err = machineid.ProtectedID("Zeronet")
	if err != nil {
		return nil, fmt.Errorf("error generating machine ID: %s", err.Error())
	}

	host, err := os.Hostname()
	if err != nil {
		log.Printf("error getting hostname: %s", err.Error())
		host = fmt.Sprintf("node-%d", rand.Uint32())
		log.Printf("defaulting hostname to %s", host)
	} else {
		hostname := strings.Split(host, ".")
		node.hostname = hostname[0]
	}

	node.grpcClient, err = NewControllerClient(controller)
	if err != nil {
		return nil, err
	}

	// TODO make this configurable
	node.parseStunUrls(
		"stun:stun.l.google.com:19302",
		"stun:stun1.l.google.com:19302",
		"stun:stun.services.mozilla.com:3478",
		"stun:stun.siptraffic.com:3478",
	)

	return node, nil
}

func (n *Node) Start() error {
	var err error

	loggedIn := n.loggedIn.Load()
	if !loggedIn {
		return errors.New("node is not logged in")
	}

	running := n.running.Load()
	if running {
		return fmt.Errorf("node is already running")
	}

	// TODO SO_REUSEPORT is a pipedream on mac
	// Linux kernel does provide load balancing on each opened fd
	// under SO_REUSEPORT but darwin doesn't so far

	n.conn, err = conn.NewConn(n.port)
	if err != nil {
		return err
	}

	n.udpMux = ice.NewUniversalUDPMuxDefault(ice.UniversalUDPMuxParams{
		Logger:                nil,
		UDPConn:               n.conn.GetConn(),
		XORMappedAddrCacheTTL: time.Second * 20,
	})

	// Create local tunnel interface
	n.tun, err = tun.NewTun()
	if err != nil {
		n.conn.Close()
		return err
	}

	n.runCtx, n.runCancel = context.WithCancel(context.Background())

	err = n.Run()
	if err != nil {
		n.udpMux.Close()
		n.conn.Close()
		n.tun.Close()
		// Invalidate context since not running
		n.runCancel()
		return err
	}

	return nil
}

func (node *Node) Run() error {
	if node.conn == nil || node.tun == nil {
		return fmt.Errorf("node connections have not been initialized")
	}

	if node.udpMux.IsClosed() {
		return fmt.Errorf("node udp mux is closed")
	}

	// Configure tunnel ip/routes
	err := node.tun.ConfigureIPAddress(node.ip)
	if err != nil {
		return err
	}

	//// Initially set endpoint
	//err = node.sendStunRequest()
	//if err != nil {
	//	return errors.New("error sending stun request: " + err.Error())
	//}

	//go node.ReadUDPPackets(node.OnUDPPacket, 0)

	node.StartUpdateStream(node.runCtx)

	go node.ReadTunPackets(node.OnTunnelPacket)

	//go node.stunRoutine()
	//for _, peer := range node.maps.id {
	//	if peer.running.Load() {
	//		peer.cancel()
	//	}
	//}

	node.running.Store(true)
	return nil
}

func (node *Node) Stop() error {
	if node.conn == nil || node.tun == nil {
		return fmt.Errorf("node connections have not been initialized, so not running")
	}

	running := node.running.Load()
	if !running {
		return fmt.Errorf("node is already stopped")
	}

	node.StopAllPeers()
	node.runCancel()
	node.udpMux.Close()
	node.conn.Close()
	node.tun.Close()

	node.running.Store(false)
	//node.grpcClient.Close()
	//node.grpcClient = nil
	return nil
}

func (node *Node) StopAllPeers() {
	node.maps.l.RLock()
	defer node.maps.l.RUnlock()

	for _, peer := range node.maps.id {
		go peer.Stop()
	}
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
		// TODO: Possibly STUN message
		if isStunMessage(buffer.in) {
			node.handleStunMessage(buffer.in)
		} else {
			log.Println(err)
		}

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

		peer.InboundPacket(buffer)
		return
	// Remote peer sent punch packet
	case header.Punch:
		log.Printf("[inbound] received punch packet from peer %d", sender)
		PutInboundBuffer(buffer)
		return
	case header.Discovery:
		// Logic to process stun/discovery responses
		//node.HandleDiscoveryResponse(buffer)
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

func (node *Node) getAgentConfig() *ice.AgentConfig {
	return &ice.AgentConfig{
		UDPMux:       node.udpMux.UDPMuxDefault,
		UDPMuxSrflx:  node.udpMux,
		NetworkTypes: []ice.NetworkType{ice.NetworkTypeUDP4},
		Urls:         node.stunUrls,
	}
}

func (node *Node) parseStunUrls(urls ...string) {
	var stunUrls []*stun.URI

	for _, u := range urls {
		parsed, err := stun.ParseURI(u)
		if err != nil {
			log.Printf("failed to parse stun uri: %s", u)
			continue
		}
		stunUrls = append(stunUrls, parsed)
	}

	node.stunUrls = stunUrls
}
