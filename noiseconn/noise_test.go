package noiseconn

import (
	"crypto/rand"
	"testing"

	"github.com/flynn/noise"
)

func TestNoiseHandshake(t *testing.T) {
	cs := noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2s)
	kp1, err := cs.GenerateKeypair(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	kp2, err := cs.GenerateKeypair(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	ns1 := NewNoiseState(kp1, kp2.Public)
	if err != nil {
		t.Fatal(err)
	}
	ns2 := NewNoiseState(kp2, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err = ns1.Initialize(true); err != nil {
		t.Fatal(err)
	}

	if err = ns2.Initialize(false); err != nil {
		t.Fatal(err)
	}

	b1 := make([]byte, 1400)
	b2 := make([]byte, 1400)

	p1, err := ns1.GenerateHandshakeP1(b1[:0])
	if err != nil {
		t.Fatal(err)
	}

	err = ns2.ConsumeHandshakeP1(p1)
	if err != nil {
		t.Fatal(err)
	}

	p2, err := ns2.GenerateHandshakeP2(b2[:0])
	if err != nil {
		t.Fatal(err)
	}

	err = ns1.ConsumeHandshakeP2(p2)
	if err != nil {
		t.Fatal(err)
	}
}
