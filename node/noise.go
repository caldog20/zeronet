package node

import "github.com/flynn/noise"

var (
	CipherSuite = noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2s)
	BaseConfig  = noise.Config{CipherSuite: CipherSuite, Pattern: noise.HandshakeIK}
)

func CreateHandshake(initiator bool, keypair noise.DHKey, rs []byte) (*noise.HandshakeState, error) {
	config := BaseConfig

	if initiator {
		config.Initiator = true
		config.PeerStatic = rs
	} else {
		config.Initiator = false
		config.PeerStatic = nil
	}

	config.StaticKeypair = keypair

	handshake, err := noise.NewHandshakeState(config)
	if err != nil {
		return nil, err
	}

	return handshake, nil
}
