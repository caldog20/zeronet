package node

import (
	"context"
	"errors"
	"log"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	proto "github.com/caldog20/zeronet/proto/gen/controller/v1"
	"github.com/pion/ice/v3"
)

const (
	// Timers
	TimerHandshakeTimeout = time.Second * 5
	TimerRxTimeout        = time.Second * 15
	TimerKeepalive        = time.Second * 10
	// Counts
	CountHandshakeRetries = 10

	InboundChannelSize   = 1024
	OutboundChannelSize  = 1024
	HandshakeChannelSize = 3
)

// TODO proper self-contained state machine for noise handshakes
type Peer struct {
	mu          sync.RWMutex
	pendingLock sync.RWMutex
	Hostname    string

	agent *ice.Agent
	conn  *ice.Conn
	node  *Node // Pointer back to node for stuff
	IP    netip.Addr
	ID    uint32

	outbound      chan *OutboundBuffer
	iceCreds      chan string
	iceCandidates chan ice.Candidate

	running     atomic.Bool
	inTransport atomic.Bool
	connecting  atomic.Bool

	wg sync.WaitGroup
}

func NewPeer() *Peer {
	peer := new(Peer)

	// channels
	peer.outbound = make(chan *OutboundBuffer, OutboundChannelSize) // allow up to 64 packets to be cached/pending handshake???
	peer.iceCreds = make(chan string, 2)
	peer.iceCandidates = make(chan ice.Candidate)
	peer.wg = sync.WaitGroup{}

	// peer.ctx, peer.cancel = context.WithCancel(context.Background())
	return peer
}

// TODO Proper error text for context around the issue
func (node *Node) AddPeer(peerInfo *proto.Peer) (*Peer, error) {
	peer := NewPeer()

	peer.mu.Lock()
	defer peer.mu.Unlock()

	var err error

	peer.node = node
	peer.agent, err = ice.NewAgent(node.getAgentConfig())

	err = peer.agent.OnCandidate(func(c ice.Candidate) {
		if c == nil {
			return
		}
		node.sendPeerCandidate(peer.ID, c.Marshal())
	})

	if err != nil {
		return nil, err
	}

	err = peer.agent.OnConnectionStateChange(func(c ice.ConnectionState) {
		switch c {
		case ice.ConnectionStateConnected:
			peer.connecting.Store(false)
			peer.inTransport.Store(true)
			peer.pendingLock.Unlock()
			log.Printf("peer %d connected", peer.ID)
		case ice.ConnectionStateDisconnected:
			peer.inTransport.Store(false)
			peer.pendingLock.Lock()
			log.Printf("peer %d disconnected", peer.ID)
		case ice.ConnectionStateFailed:
			peer.connecting.Store(false)
			peer.inTransport.Store(false)
			log.Printf("peer %d ice connection failed", peer.ID)
		case ice.ConnectionStateClosed:
			log.Printf("peer %d closed agent", peer.ID)
		case ice.ConnectionStateCompleted:
		default:
		}
	})

	if err != nil {
		return nil, err
	}

	// TODO Fix this
	peer.ID = peerInfo.Id
	peer.IP, err = ParseAddr(peerInfo.Ip)
	if err != nil {
		return nil, err
	}

	peer.Hostname = peerInfo.Hostname

	// TODO Add methods to manipulate map
	node.maps.l.Lock()
	defer node.maps.l.Unlock()
	node.maps.id[peer.ID] = peer
	node.maps.ip[peer.IP] = peer

	return peer, nil
}

func (peer *Peer) Start() error {
	peer.mu.Lock()
	defer peer.mu.Unlock()

	log.Printf("Starting peer %d", peer.ID)

	// Peer is already running
	if peer.running.Load() {
		return errors.New("peer already running")
	}

	// Lock here when starting peer so routines have to wait for handshake before trying to read data from channels
	peer.pendingLock.Lock()

	//peer.wg.Add(2)
	//go peer.Inbound()
	//go peer.Outbound()

	peer.running.Store(true)
	peer.inTransport.Store(false)
	return nil
}

func (peer *Peer) Stop() {
	log.Printf("Stopping peer %d", peer.ID)
	peer.running.Store(false)
	peer.ResetState()

	// send nil value to kill goroutines
	peer.mu.Lock()
	defer peer.mu.Unlock()
	peer.outbound <- nil

	// Wait until all routines are finished
	peer.wg.Wait()
	log.Printf("peer %d goroutines have stopped", peer.ID)
	peer.pendingLock.Unlock()
}

func (peer *Peer) InboundPacket(buffer *InboundBuffer) {
	if !peer.running.Load() {
		PutInboundBuffer(buffer)
		return
	}
}

func (peer *Peer) OutboundPacket(buffer *OutboundBuffer) {
	if !peer.running.Load() {
		PutOutboundBuffer(buffer)
		return
	}

	// For tracking full channels
	select {
	case peer.outbound <- buffer:
	default:
		log.Printf("peer id %d: outbound channel full", peer.ID)
	}

	if !peer.inTransport.Load() && !peer.connecting.Load() {
		peer.InitiateConnection()
	}
	//if !peer.inTransport.Load() && peer.noise.state.Load() == 0 {
	//	peer.TrySendHandshake(false)
	//}
}

func (peer *Peer) InitiateConnection() {
	log.Println("Initiating connection")
	peer.connecting.Store(true)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
		defer cancel()

		localUfrag, localPwd, err := peer.agent.GetLocalUserCredentials()
		if err != nil {
			log.Println("error getting local user credentials: ", err)
			return
		}

		peer.node.sendPeerIceOffer(peer.ID, localUfrag, localPwd)
		rUfrag := <-peer.iceCreds
		rPwd := <-peer.iceCreds
		log.Println("received creds ", rUfrag, rPwd)

		if err = peer.agent.GatherCandidates(); err != nil {
			log.Println("error gathering candidates: ", err)
			return
		}

		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case c := <-peer.iceCandidates:
					peer.agent.AddRemoteCandidate(c)
				}
			}
		}()

		peer.conn, err = peer.agent.Dial(ctx, rUfrag, rPwd)
		if err != nil {
			log.Println("error Dialing remote peer: ", err)
			peer.conn = nil
			return
		}
	}()
}

func (peer *Peer) RespondConnection(ufrag, pwd string) {
	log.Println("Responding connection")
	if peer.inTransport.Load() {
		return
	}
	peer.connecting.Store(true)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()

		localUfrag, localPwd, err := peer.agent.GetLocalUserCredentials()
		if err != nil {
			log.Println("error getting local user credentials: ", err)
			return
		}

		peer.node.sendPeerIceAnswer(peer.ID, localUfrag, localPwd)

		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case c := <-peer.iceCandidates:
					peer.agent.AddRemoteCandidate(c)
				}
			}
		}()

		if err = peer.agent.GatherCandidates(); err != nil {
			log.Println("error gathering candidates: ", err)
			return
		}
		peer.conn, err = peer.agent.Accept(ctx, ufrag, pwd)
		if err != nil {
			log.Println("error Accepting remote peer: ", err)
			peer.conn = nil
			return
		}
	}()
}

func (peer *Peer) ResetState() {
	// Temporarily stop peer while resetting state
	// to prevent peer trying to process packets while clearing
	// If peer was running, reset the running value after state is cleared
	wasRunning := peer.running.CompareAndSwap(true, false)
	if wasRunning {
		defer func() {
			peer.running.Store(true)
		}()
	}

	peer.mu.Lock()
	defer peer.mu.Unlock()

	peer.inTransport.Store(false)
}

func (peer *Peer) flushOutboundQueue() {
LOOP:
	for {
		select {
		case b, ok := <-peer.outbound:
			if !ok {
				break LOOP
			}
			PutOutboundBuffer(b)
		default:
			break LOOP
		}
	}
}
