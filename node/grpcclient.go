package node

import (
	"context"
	"crypto/subtle"
	"errors"
	"io"
	"log"
	"net"
	"time"

	"github.com/caldog20/zeronet/node/conn"
	"github.com/caldog20/zeronet/pkg/header"
	controllerv1 "github.com/caldog20/zeronet/proto/gen/controller/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ControllerClient struct {
	client controllerv1.ControllerServiceClient
	conn   *grpc.ClientConn
}

func NewControllerClient(address string) (*ControllerClient, error) {
	dialCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return nil, errors.New("error connecting to controller grpc server at: " + address)
	}

	client := controllerv1.NewControllerServiceClient(conn)
	return &ControllerClient{client, conn}, nil
}

func (c *ControllerClient) Close() error {
	c.client = nil
	return c.conn.Close()
}

func (c *ControllerClient) UpdateEndpoint(id string, endpoint string) {
	_, err := c.client.UpdateEndpoint(context.Background(), &controllerv1.UpdateEndpointRequest{MachineId: id, Endpoint: endpoint})
	if err != nil {
		log.Printf("error updating endpoint: %v", err)
	}
}

//func (node *Node) Login() error {
//	node.noise.l.Lock()
//	defer node.noise.l.Unlock()
//
//	pubkey := base64.StdEncoding.EncodeToString(node.noise.keyPair.Public)
//	login, err := node.controller.LoginPeer(
//		context.TODO(),
//		&controllerv1.LoginRequest{PublicKey: pubkey},
//	)
//	if err != nil {
//		return err
//	}
//
//	node.id = login.Config.Id
//	node.ip = netip.MustParsePrefix(login.Config.TunnelIp)
//
//	return nil
//}

//	func (node *Node) SetRemoteEndpoint(endpoint string) error {
//		_, err := node.controller.SetPeerEndpoint(context.TODO(), &controllerv1.Endpoint{
//			Id:       node.id,
//			Endpoint: endpoint,
//		})
//		if err != nil {
//			return err
//		}
//		return nil
//	}
//
//	func (node *Node) Register() error {
//		node.noise.l.RLock()
//		defer node.noise.l.RUnlock()
//		pubkey := base64.StdEncoding.EncodeToString(node.noise.keyPair.Public)
//
//		regmsg := &controllerv1.RegisterRequest{
//			PublicKey:   pubkey,
//			RegisterKey: "registermeplz!",
//		}
//
//		_, err := node.controller.RegisterPeer(context.TODO(), regmsg)
//		if err != nil {
//			return err
//		}
//
//		return nil
//	}
//
// TODO: Move some of the stream logic to ControllerClient
func (node *Node) StartUpdateStream(ctx context.Context) {
	stream, err := node.grpcClient.client.UpdateStream(ctx, &controllerv1.UpdateRequest{
		MachineId: node.machineID,
	})
	if err != nil {
		log.Fatal(err)
	}

	response, err := stream.Recv()
	if err != nil {
		stream = nil
		log.Fatal(err)
	}
	node.HandleUpdate(response)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				response, err = stream.Recv()
				// TODO properly handle error
				if err != nil {
					if err == io.EOF {
						return
					}
					log.Printf("error receiving stream update response: %v", err)
					return
				}
				node.HandleUpdate(response)
			}
		}
	}()
}

func (node *Node) HandleUpdate(update *controllerv1.UpdateResponse) {
	switch update.UpdateType {
	case controllerv1.UpdateType_INIT:
		node.handleInitialSync(update)
	case controllerv1.UpdateType_CONNECT:
		node.handlePeerConnectUpdate(update)
	case controllerv1.UpdateType_DISCONNECT:
		//node.handlePeerDisconnectUpdate(update)
	case controllerv1.UpdateType_PUNCH:
		node.handlePeerPunchRequest(update)
	case controllerv1.UpdateType_LOGOUT:
		node.handleLogout()
	default:
		log.Println("unmatched update message type")
		return
	}
}

// TODO Controller is forcing peer to log out
// Node grpc service should still run, and requires a login/up to start again
func (node *Node) handleLogout() {
	err := node.Stop()
	if err != nil {
		log.Println("error stopping node during logout")
	}
	node.loggedIn.Store(false)
}

func (node *Node) handlePeerPunchRequest(update *controllerv1.UpdateResponse) {
	endpoint := update.GetPunchEndpoint()
	ua, err := net.ResolveUDPAddr(conn.UDPType, endpoint)
	if err != nil {
		log.Printf("error parsing udp punch address: %s", err)
		return
	}
	punch := make([]byte, 16)

	h := header.NewHeader()

	punch, err = h.Encode(punch, header.Punch, node.id, 0)
	if err != nil {
		log.Println("error encoding header for punch message")
	}

	node.conn.WriteToUDP(punch, ua)
	log.Printf("sent punch message to udp address: %s", ua.String())
}

func (node *Node) handleInitialSync(update *controllerv1.UpdateResponse) {
	for _, peer := range update.PeerList.Peers {
		p, err := node.AddPeer(peer)
		if err != nil {
			panic(err)
			continue
		}
		err = p.Start()
		if err != nil {
			panic(err)
		}
	}
}

func (node *Node) handlePeerConnectUpdate(update *controllerv1.UpdateResponse) {
	if update.PeerList.Count < 1 {
		return
	}
	if update.UpdateType != controllerv1.UpdateType_CONNECT {
		return
	}

	rp := update.PeerList.Peers[0]

	node.maps.l.RLock()
	p, found := node.maps.id[rp.Id]
	node.maps.l.RUnlock()
	if !found {
		peer, err := node.AddPeer(rp)
		if err != nil {
			return
		}
		err = peer.Start()
		if err != nil {
			panic(err)
		}
		return
	}

	// Peer already found, update
	err := p.Update(rp)
	if err != nil {
		panic(err)
	}
}

// // TODO Fix variable naming and compares
func (peer *Peer) Update(info *controllerv1.Peer) error {
	peer.mu.RLock()
	currentEndpoint := peer.raddr.AddrPort()
	currentKey := peer.noise.pubkey
	//currentHostname := peer.Hostname
	currentIP := peer.IP
	peer.mu.RUnlock()

	// TODO Helper function for parsing IPs
	newEndpoint, err := ParseAddrPort(info.Endpoint)
	if err != nil {
		log.Println(err)
		return err
	}

	if CompareAddrPort(currentEndpoint, newEndpoint) != 0 {
		peer.mu.Lock()
		newRemote, err := net.ResolveUDPAddr(conn.UDPType, newEndpoint.String())
		if err != nil {
			log.Println("error updating peer endpoint udp address")
		} else {
			peer.raddr = newRemote
		}
		peer.mu.Unlock()
	}

	//if strings.Compare(currentHostname, info.Hostname) != 0 {
	//	peer.mu.Lock()
	//	peer.Hostname = info.Hostname
	//	peer.mu.Unlock()
	//}

	newKey, err := DecodeBase64Key(info.PublicKey)
	if err != nil {
		log.Println(err)
		//return err
	}

	if subtle.ConstantTimeCompare(currentKey, newKey) != 1 {
		// TODO If the key has changed, we need to stop the peer and clear state,
		// update new key and restart peer completely
		panic("peer key update not yet implemented")
		peer.Stop()
		peer.mu.Lock()
		peer.noise.pubkey = newKey
		peer.mu.Unlock()
		err = peer.Start()
		if err != nil {
			panic(err)
		}
	}

	newIP, err := ParseAddr(info.Ip)
	if err != nil {
		log.Println(err)
		//return err
	}

	if currentIP.Compare(newIP) != 0 {
		peer.mu.Lock()
		peer.IP = newIP
		peer.mu.Unlock()
	}

	return nil
}

func (node *Node) RequestPunch(id uint32) {
	node.lock.RLock()
	defer node.lock.RUnlock()
	_, err := node.grpcClient.client.Punch(context.Background(), &controllerv1.PunchRequest{
		MachineId: node.machineID,
		DstPeerId: id,
		Endpoint:  node.discoveredEndpoint,
	})

	if err != nil {
		log.Printf("error requesting punch for peer id %d: %v", id, err)
	}
}
