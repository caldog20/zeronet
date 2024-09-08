package peer

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
	CipherSuite = noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2s)
	BaseConfig  = noise.Config{CipherSuite: CipherSuite, Pattern: noise.HandshakeIK}
)

type NoiseState struct {
	initiator bool
	state     atomic.Uint64
	rs        []byte
	rx        *noise.CipherState
	tx        *noise.CipherState
	hs        *noise.HandshakeState
	config    noise.Config
	nonce     atomic.Uint64
}

func NewNoiseState(s noise.DHKey, rs []byte) (*NoiseState, error) {
	config := BaseConfig
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
	}, nil
}

func (ns *NoiseState) Initialize(initiator bool) error {
	ns.initiator = initiator
	ns.config.Initiator = initiator

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
	data, err := ns.rx.Decrypt(decrypted[:0], nil, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("error decrypting message: %v", err)
	}
	return data, nil
}

func (ns *NoiseState) Encrypt(plaintext, encrypted []byte, n uint64) ([]byte, error) {
	data, err := ns.tx.Encrypt(encrypted[:0], nil, plaintext)
	if err != nil {
		return nil, fmt.Errorf("error encrypting message: %v", err)
	}
	return data, nil
}
