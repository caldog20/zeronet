package node

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	controllerv1 "github.com/caldog20/zeronet/proto/gen/controller/v1"
	"github.com/pion/ice/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
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

//func (c *ControllerClient) UpdateEndpoint(id string, endpoint string) {
//	_, err := c.client.UpdateEndpoint(context.Background(), &controllerv1.UpdateEndpointRequest{MachineId: id, Endpoint: endpoint})
//	if err != nil {
//		log.Printf("error updating endpoint: %v", err)
//	}
//}

// TODO: Move some of the stream logic to ControllerClient
func (node *Node) StartUpdateStream(ctx context.Context) error {
	sCtx := metadata.AppendToOutgoingContext(ctx, "authorization", fmt.Sprintf("Bearer %s", node.machineID))
	stream, err := node.grpcClient.client.UpdateStream(sCtx)
	if err != nil {
		return fmt.Errorf("error connecting to update stream: %v", err)
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

	go func() {
		for {
			select {
			case <-ctx.Done():
				stream.CloseSend()
				return
			case msg, ok := <-node.outboundUpdates:
				if !ok {
					log.Println("outbound updates channel closed")
					stream.CloseSend()
					return
				}
				if err := stream.Send(msg); err != nil {
					if err == io.EOF {
						return
					}
					log.Printf("error sending stream update response: %v", err)
				}
			}
		}
	}()

	return nil
}

func (node *Node) HandleUpdate(update *controllerv1.UpdateResponse) {
	switch update.UpdateType {
	case controllerv1.UpdateType_INIT:
		node.handleInitialSync(update)
	case controllerv1.UpdateType_CONNECT:
		node.handlePeerConnectUpdate(update)
	case controllerv1.UpdateType_DISCONNECT:
		//node.handlePeerDisconnectUpdate(update)
	case controllerv1.UpdateType_LOGOUT:
		node.handleLogout()
	case controllerv1.UpdateType_ICE:
		node.handleIceUpdate(update.GetIceUpdate())
	default:
		log.Println("unmatched update message type")
		return
	}
}

func (node *Node) sendPeerCandidate(id uint32, candidate string) {
	update := &controllerv1.UpdateRequest{
		UpdateType: controllerv1.UpdateType_ICE,
		IceUpdate: &controllerv1.IceUpdate{
			UpdateType: controllerv1.IceUpdateType_CANDIDATE,
			PeerId:     id,
			Candidate:  candidate,
		},
	}
	node.outboundUpdates <- update
}

func (node *Node) sendPeerIceOffer(id uint32, ufrag, pwd string) {
	update := &controllerv1.UpdateRequest{
		UpdateType: controllerv1.UpdateType_ICE,
		IceUpdate: &controllerv1.IceUpdate{
			UpdateType: controllerv1.IceUpdateType_OFFER,
			PeerId:     id,
			Ufrag:      ufrag,
			Pwd:        pwd,
		},
	}
	node.outboundUpdates <- update
}

func (node *Node) sendPeerIceAnswer(id uint32, ufrag, pwd string) {
	update := &controllerv1.UpdateRequest{
		UpdateType: controllerv1.UpdateType_ICE,
		IceUpdate: &controllerv1.IceUpdate{
			UpdateType: controllerv1.IceUpdateType_ANSWER,
			PeerId:     id,
			Ufrag:      ufrag,
			Pwd:        pwd,
		},
	}
	node.outboundUpdates <- update
}

func (node *Node) handleIceUpdate(update *controllerv1.IceUpdate) {
	peer, found := node.lookupPeer(update.GetPeerId())
	if !found {
		log.Printf("peer %s not found for ice update", update.GetPeerId())
		return
	}

	log.Printf("peer %s found for ice update: %v", update.GetPeerId(), update)

	switch update.UpdateType {
	case controllerv1.IceUpdateType_OFFER:
		peer.iceCreds <- update.GetUfrag()
		peer.iceCreds <- update.GetPwd()
	case controllerv1.IceUpdateType_ANSWER:
		peer.RespondConnection(update.GetUfrag(), update.GetPwd())
	case controllerv1.IceUpdateType_CANDIDATE:
		cand, err := ice.UnmarshalCandidate(update.GetCandidate())
		if err != nil {
			log.Printf("error unmarshaling candidate: %v", err)
			return
		}
		peer.iceCandidates <- cand
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

	rp := update.PeerList.Peers[0]

	node.maps.l.RLock()
	_, found := node.maps.id[rp.Id]
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
	log.Println("skipping peer update TODO")
	// Peer already found, update
	//err := p.Update(rp)
	//if err != nil {
	//	panic(err)
	//}
}

// // TODO Fix variable naming and compares
//func (peer *Peer) Update(info *controllerv1.Peer) error {
//	peer.mu.RLock()
//	currentEndpoint := peer.raddr.AddrPort()
//	//currentKey := peer.noise.pubkey
//	currentHostname := peer.Hostname
//	currentIP := peer.IP
//	peer.mu.RUnlock()
//
//	// TODO Helper function for parsing IPs
//	newEndpoint, err := ParseAddrPort(info.Endpoint)
//	if err != nil {
//		log.Println(err)
//		return err
//	}
//
//	if CompareAddrPort(currentEndpoint, newEndpoint) != 0 {
//		peer.mu.Lock()
//		newRemote, err := net.ResolveUDPAddr(conn.UDPType, newEndpoint.String())
//		if err != nil {
//			log.Println("error updating peer endpoint udp address")
//		} else {
//			peer.raddr = newRemote
//		}
//		peer.mu.Unlock()
//	}
//
//	if strings.Compare(currentHostname, info.Hostname) != 0 {
//		peer.mu.Lock()
//		peer.Hostname = info.Hostname
//		peer.mu.Unlock()
//	}
//
//	newKey, err := DecodeBase64Key(info.PublicKey)
//	if err != nil {
//		log.Println(err)
//		//return err
//	} else {
//		if subtle.ConstantTimeCompare(currentKey, newKey) != 1 {
//			// TODO If the key has changed, we need to stop the peer and clear state,
//			// update new key and restart peer completely
//			//panic("peer key update not yet implemented")
//			peer.Stop()
//			peer.mu.Lock()
//			peer.noise.pubkey = newKey
//			peer.mu.Unlock()
//			err = peer.Start()
//			if err != nil {
//				panic(err)
//			}
//		}
//	}
//
//	newIP, err := ParseAddr(info.Ip)
//	if err != nil {
//		log.Println(err)
//		//return err
//	}
//
//	if currentIP.Compare(newIP) != 0 {
//		peer.mu.Lock()
//		peer.IP = newIP
//		peer.mu.Unlock()
//	}
//
//	return nil
//}
