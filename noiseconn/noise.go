package noiseconn

import (
	"fmt"
	"sync/atomic"

	"github.com/flynn/noise"
)

const (
	None uint64 = iota
	HandshakeSent
	HandshakeReceived
	HandshakeComplete
)

var (
// CipherSuite = noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2s)
// BaseConfig  = noise.Config{CipherSuite: CipherSuite, Pattern: noise.HandshakeIK}
)

type NoiseState struct {
	state     atomic.Uint64
	rx        *noise.CipherState
	tx        *noise.CipherState
	hs        *noise.HandshakeState
	config    noise.Config
	nonce     atomic.Uint64
}

func NewNoiseState(s noise.DHKey, rs []byte) *NoiseState {
	cs := noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2s)
	config := noise.Config{CipherSuite: cs, Pattern: noise.HandshakeIK}
	config.PeerStatic = rs
	config.StaticKeypair = s
	//
	// hs, err := noise.NewHandshakeState(config)
	// if err != nil {
	// 	return nil, err
	// }

	return &NoiseState{
		hs:     nil,
		rx:     nil,
		tx:     nil,
		config: config,
	}
}

func (ns *NoiseState) Initialize(initiator bool) error {
	ns.config.Initiator = initiator
  
  if !initiator {
    ns.config.PeerStatic = nil
  }

	hs, err := noise.NewHandshakeState(ns.config)
	if err != nil {
		return err
	}

	ns.hs = hs
	ns.rx = nil
	ns.tx = nil

	return nil
}

func (ns *NoiseState) Reset() {
	ns.hs = nil
	ns.rx = nil
	ns.tx = nil
	ns.nonce.Store(0)
	ns.state.Store(None)
}

// Construct handshake message p1 as initiator
func (ns *NoiseState) GenerateHandshakeP1(out []byte) ([]byte, error) {
	msg, _, _, err := ns.hs.WriteMessage(out, nil)
	if err != nil {
		return nil, fmt.Errorf("error writing handshake p1 message: %v", err)
	}

	ns.state.Store(HandshakeSent)
	return msg, nil
}

// Consume handshake message p1 as responder
func (ns *NoiseState) ConsumeHandshakeP1(in []byte) error {
	_, _, _, err := ns.hs.ReadMessage(nil, in)
	if err != nil {
		return fmt.Errorf("error reading handshake p1 message: %v", err)
	}
	ns.state.Store(HandshakeReceived)
	return nil
}

// Construct handshake message p2 as responder
func (ns *NoiseState) GenerateHandshakeP2(out []byte) ([]byte, error) {
	msg, rx, tx, err := ns.hs.WriteMessage(out, nil)
	if err != nil {
		return nil, fmt.Errorf("error writing handshake p2 message: %v", err)
	}
	ns.rx = rx
	ns.tx = tx

	ns.state.Store(HandshakeComplete)
	return msg, nil
}

// Consume handshake message p2 as initiator
func (ns *NoiseState) ConsumeHandshakeP2(in []byte) error {
	_, tx, rx, err := ns.hs.ReadMessage(nil, in)
	if err != nil {
		return fmt.Errorf("error reading handshake p2 message: %v", err)
	}

	ns.rx = rx
	ns.tx = tx

	ns.state.Store(HandshakeComplete)
	return nil
}

func (ns *NoiseState) Decrypt(ciphertext, decrypted []byte, n uint64) ([]byte, error) {
	data, err := ns.rx.Decrypt(decrypted, nil, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("error decrypting message: %v", err)
	}
	ns.nonce.Add(1)
	return data, nil
}

func (ns *NoiseState) Encrypt(plaintext, encrypted []byte, n uint64) ([]byte, error) {
	data, err := ns.tx.Encrypt(encrypted, nil, plaintext)
	if err != nil {
		return nil, fmt.Errorf("error encrypting message: %v", err)
	}
	ns.nonce.Add(1)
	return data, nil
}

func (ns *NoiseState) Nonce() uint64 {
	return ns.nonce.Load()
}
