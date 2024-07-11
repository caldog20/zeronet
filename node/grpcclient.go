package node

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	controllerv1 "github.com/caldog20/zeronet/proto/gen/controller/v1"
	"github.com/pion/ice/v3"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type ControllerClient struct {
	client    controllerv1.ControllerServiceClient
	conn      *grpc.ClientConn
	rxUpdates chan *controllerv1.UpdateResponse
	txUpdates chan *controllerv1.UpdateRequest
}

func NewControllerClient(address string) (*ControllerClient, error) {
	dialCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return nil, errors.New("error connecting to controller grpc server at: " + address)
	}
	rxUpdates := make(chan *controllerv1.UpdateResponse, 5)
	txUpdates := make(chan *controllerv1.UpdateRequest, 5)
	client := controllerv1.NewControllerServiceClient(conn)
	return &ControllerClient{client, conn, rxUpdates, txUpdates}, nil
}

func (c *ControllerClient) Close() error {
	close(c.rxUpdates)
	close(c.txUpdates)
	return c.conn.Close()
}

func (c *ControllerClient) waitForConnectivityReady(ctx context.Context) bool {
	if c.conn.GetState() == connectivity.Ready {
		return true
	} else {
		state := c.conn.GetState()
		if state != connectivity.Connecting {
			c.conn.Connect()
		}
		return c.conn.WaitForStateChange(ctx, c.conn.GetState())
	}
}

func (c *ControllerClient) ConnectStream(ctx context.Context) (controllerv1.ControllerService_UpdateStreamClient, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, nil
		default:
			ready := c.waitForConnectivityReady(ctx)
			if !ready {
				log.Println("error connecting to controller grpc server, trying again")
				continue
			}
			stream, err := c.client.UpdateStream(ctx)
			if err != nil {
				return nil, err
			}
			return stream, nil
		}
	}
}

//func (c *ControllerClient) UpdateEndpoint(id string, endpoint string) {
//	_, err := c.client.UpdateEndpoint(context.Background(), &controllerv1.UpdateEndpointRequest{MachineId: id, Endpoint: endpoint})
//	if err != nil {
//		log.Printf("error updating endpoint: %v", err)
//	}
//}

func (c *ControllerClient) RunUpdateStream(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			stream, err := c.ConnectStream(ctx)
			if err != nil {
				log.Printf("error connecting to update stream: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}
			eg, egCtx := errgroup.WithContext(ctx)

			eg.Go(func() error {
				for {
					select {
					case <-egCtx.Done():
						stream.CloseSend()
						return egCtx.Err()
					case msg, ok := <-c.txUpdates:
						if !ok {
							log.Println("outbound updates channel closed")
							stream.CloseSend()
							return errors.New("outbound updates channel closed")
						}
						if err := stream.Send(msg); err != nil {
							log.Printf("error sending stream update response: %v", err)
							stream.CloseSend()
							return err
						}
					}
				}
			})

			eg.Go(func() error {
				for {
					select {
					case <-egCtx.Done():
						return egCtx.Err()
					default:
						response, err := stream.Recv()
						// TODO properly handle error
						if err != nil {
							code, msg := getErrorFromStatus(err)
							if code == codes.Canceled {
								return err
							} else {
								log.Printf("error receiving stream update response: %v", msg)
								return err
							}
						}
						c.rxUpdates <- response
					}
				}
			})

			eg.Wait()
		}
	}
}

func (c *ControllerClient) SubmitUpdate(update *controllerv1.UpdateRequest) {
	c.txUpdates <- update
}

func getErrorFromStatus(err error) (codes.Code, string) {
	s, ok := status.FromError(err)
	if ok {
		return s.Code(), s.Message()
	} else {
		return codes.Unknown, "unknown error"
	}
}

// TODO: Fix stream auth
func (node *Node) StartUpdateStream(ctx context.Context) {
	sCtx := metadata.AppendToOutgoingContext(ctx, "authorization", fmt.Sprintf("Bearer %s", node.machineID))
	go node.grpcClient.RunUpdateStream(sCtx)
	go node.HandleUpdates(ctx)
}

func (node *Node) HandleUpdates(ctx context.Context) {
	for update := range node.grpcClient.rxUpdates {
		select {
		case <-ctx.Done():
			return
		default:
		}
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
	node.grpcClient.SubmitUpdate(update)
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
	node.grpcClient.SubmitUpdate(update)
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
	node.grpcClient.SubmitUpdate(update)
}

func (node *Node) handleIceUpdate(update *controllerv1.IceUpdate) {
	peer, found := node.lookupPeer(update.GetPeerId())
	if !found {
		log.Printf("peer %s not found for ice update", update.GetPeerId())
		return
	}

	log.Printf("peer %s found for ice update: %v", update.GetPeerId(), update)

	switch update.UpdateType {
	case controllerv1.IceUpdateType_ANSWER:
		peer.iceCredentials <- IceCreds{update.GetUfrag(), update.GetPwd()}
	case controllerv1.IceUpdateType_OFFER:
		peer.RespondConnection(IceCreds{update.GetUfrag(), update.GetPwd()})
	case controllerv1.IceUpdateType_CANDIDATE:
		cand, err := ice.UnmarshalCandidate(update.GetCandidate())
		if err != nil {
			log.Printf("error unmarshaling candidate: %v", err)
			return
		}
		peer.iceCandidates <- cand
	default:
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
