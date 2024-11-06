package noiseconn

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/caldog20/zeronet/pkg/header"
	"github.com/flynn/noise"
)

const (
	HandshakeSendRetries int    = 3
	StateIdle            uint64 = 0
	StateDialing         uint64 = 1
	StateAccepting       uint64 = 2
	StateComplete        uint64 = 3
)

// type UnderlyingConn interface {
// 	// BytesReceived() uint64
// 	// BytesSent() uint64
// 	Close() error
// 	LocalAddr() net.Addr
// 	Read(p []byte) (int, error)
// 	RemoteAddr() net.Addr
// 	SetDeadline(time.Time) error
// 	SetReadDeadline(time.Time) error
// 	SetWriteDeadline(time.Time) error
// 	Write(p []byte) (int, error)
// }

type Conn struct {
	conn net.Conn

	ns           *NoiseState
	state        atomic.Uint64
	mu           sync.RWMutex
	initiator    bool
	keypair      noise.DHKey
	remoteStatic []byte
	cs           noise.CipherSuite
}

func NewNoiseConn(keypair noise.DHKey, remoteStatic []byte) *Conn {
	cs := noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2s)

	return &Conn{
		conn:         nil,
		state:        atomic.Uint64{},
		mu:           sync.RWMutex{},
		initiator:    false,
		cs:           cs,
		keypair:      keypair,
		remoteStatic: remoteStatic,
	}
}

func (nc *Conn) Reset() {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	nc.ns.Reset()
	nc.initiator = false
	nc.state.Store(StateIdle)
}

func (nc *Conn) resetLocked() {
	nc.ns.Reset()
	nc.initiator = false
	nc.state.Store(StateIdle)
}

func (nc *Conn) SetConn(conn net.Conn) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	nc.conn = conn
}

func (nc *Conn) Dial() error {
	state := nc.state.Load()

	if state != StateIdle {
		return fmt.Errorf(
			"noiseconn must be idle before attempting to call dial: current state: %s",
			getStateAsString(state),
		)
	}
  nc.state.Store(StateDialing)

  nc.mu.Lock()
  defer nc.mu.Unlock()

	err := nc.connect(true)
	if err != nil {
		return err
	}

	return nil
}

func (nc *Conn) Accept() error {

	state := nc.state.Load()
	if state != StateIdle {
		return fmt.Errorf(
			"noiseconn must be idle before attempting to call accept: current state: %s",
			getStateAsString(state),
		)
	}

	nc.state.Store(StateAccepting)

  nc.mu.Lock()
  defer nc.mu.Unlock()

	err := nc.connect(false)
	if err != nil {
		return err
	}

	return nil
}

func (nc *Conn) connect(initiator bool) error {
	// nc.mu.Lock()
	// defer nc.mu.Unlock()

	nc.initiator = initiator

	nc.ns = NewNoiseState(nc.keypair, nc.remoteStatic)
	err := nc.ns.Initialize(initiator)
	if err != nil {
		return err
	}

	if initiator {
		err = nc.initiateHandshake()
	} else {
		err = nc.waitForHandshake()
	}

	if err != nil {
		log.Printf("error establishing noise connection: %s", err)
		nc.resetLocked()
		return err
	}

	log.Printf("noise connection established")
	nc.state.Store(StateComplete)
	return nil
}

func (nc *Conn) initiateHandshake() error {
	h := header.NewHeader()
	attempts := 0

RETRY:
	data := make([]byte, 1400)
	attempts++

	packet, err := h.Encode(data, header.Handshake, 100, 0)
	if err != nil {
		return err
	}

	packet, err = nc.ns.GenerateHandshakeP1(packet)
	if err != nil {
		return err
	}

	nc.conn.SetWriteDeadline(time.Now().Add(time.Second * 3))
	_, err = nc.conn.Write(packet)
	if err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			if attempts >= 3 {
				return fmt.Errorf("exceeded handshake attempts: %s", err)
			}
			goto RETRY
		}
		return err
	}

	in := make([]byte, 1400)
	nc.conn.SetReadDeadline(time.Now().Add(time.Second * 3))
	n, err := nc.conn.Read(in)
	if err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			if attempts >= 3 {
				return fmt.Errorf("exceeded handshake attempts: %s", err)
			}
			goto RETRY
		}
		return err
	}

	err = h.Parse(data)
	if err != nil {
		if attempts >= 3 {
			return fmt.Errorf("exceeded handshake attempts: %s", err)
		}
		goto RETRY
	}

	if h.Type != header.Handshake && h.Counter != 1 {
		if attempts >= 3 {
			return fmt.Errorf("exceeded handshake attempts: invalid packet for handshake")
		}
		goto RETRY
	}

	err = nc.ns.ConsumeHandshakeP2(in[header.HeaderLen:n])
	if err != nil {
		if attempts >= 3 {
			return fmt.Errorf("exceeded handshake attempts: %s", err)
		}
		goto RETRY
	}

	nc.conn.SetDeadline(time.Time{})
	return nil
}

func (nc *Conn) waitForHandshake() error {
	h := header.NewHeader()
	attempts := 0

RETRY:
	data := make([]byte, 1400)
	attempts++

	// nc.conn.SetReadDeadline(time.Now().Add(time.Second * 3))
	n, err := nc.conn.Read(data)
	if err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			if attempts >= 3 {
				return fmt.Errorf("exceeded handshake attempts: %s", err)
			}
			goto RETRY
		}
		return err
	}

	err = h.Parse(data)
	if err != nil {
		if attempts >= 3 {
			return fmt.Errorf("exceeded handshake attempts: %s", err)
		}
		goto RETRY
	}

	if h.Type != header.Handshake && h.Counter != 0 {
		if attempts >= 3 {
			return fmt.Errorf("exceeded handshake attempts: invalid packet for handshake")
		}
		goto RETRY
	}

	err = nc.ns.ConsumeHandshakeP1(data[header.HeaderLen:n])
	if err != nil {
		if attempts >= 3 {
			return fmt.Errorf("exceeded handshake attempts: %s", err)
		}
		goto RETRY
	}

	clear(data)
	data, err = h.Encode(data, header.Handshake, 100, 1)
	if err != nil {
		return err
	}

	data, err = nc.ns.GenerateHandshakeP2(data)
	if err != nil {
		return err
	}

	nc.conn.SetWriteDeadline(time.Now().Add(time.Second * 3))
	_, err = nc.conn.Write(data)
	if err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			if attempts >= 3 {
				return fmt.Errorf("exceeded handshake attempts: %s", err)
			}
			goto RETRY
		}
		return err
	}

	nc.conn.SetDeadline(time.Time{})
	return nil
}

// TODO: Handle receiving handshake packets during active session
// TODO: Handle rekeys during active session
func (nc *Conn) Read(p []byte) (int, error) {
	if nc.ns == nil {
		return 0, errors.New("noise state not initialized")
	}
	if nc.conn == nil {
		return 0, errors.New("noise underlying conn is nil")
	}
	if nc.ns.state.Load() != HandshakeComplete {
		return 0, errors.New("noise state not ready: handshake is not complete")
	}

	nc.mu.RLock()
	defer nc.mu.RUnlock()

	h := header.NewHeader()
	data := make([]byte, 1400)
	n, err := nc.conn.Read(data)
	if err != nil {
		return n, err
	}

	err = h.Parse(data)
	if err != nil {
		return n, err
	}

	if h.Type != header.Data {
		return n, errors.New("packet is not a data packet")
	}

	p, err = nc.ns.Decrypt(data[header.HeaderLen:n], p[:0], h.Counter)
	if err != nil {
		return n, err
	}

	return n, nil
}

func (nc *Conn) Write(p []byte) (int, error) {
	if nc.ns == nil {
		return 0, errors.New("noise state not initialized")
	}
	if nc.conn == nil {
		return 0, errors.New("noise underlying conn is nil")
	}
	if nc.ns.state.Load() != HandshakeComplete {
		return 0, errors.New("noise state not ready: handshake is not complete")
	}

	nc.mu.RLock()
	defer nc.mu.RUnlock()

	h := header.NewHeader()
	data := make([]byte, 1400)

	packet, err := h.Encode(data, header.Data, 100, nc.ns.Nonce())
	if err != nil {
		return 0, err
	}

	packet, err = nc.ns.Encrypt(p, packet, 0)
	if err != nil {
		return 0, err
	}

	n, err := nc.conn.Write(packet)
	if err != nil {
		return n, err
	}

	return n, nil
}

// Close closes the noise connection, resetting the state, and
// requiring a new conn to be set and a new Dial or Accept call to establish a
// new connection.
func (nc *Conn) Close() error {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	nc.resetLocked()
	nc.conn = nil

	return nil
}

func getStateAsString(state uint64) string {
	switch state {
	case StateIdle:
		return "Idle"
	case StateDialing:
		return "Dialing"
	case StateAccepting:
		return "Accepting"
	case StateComplete:
		return "Connected"
	default:
		return "Unknown"
	}
}
