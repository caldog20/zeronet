package peer

import (
	"crypto/rand"
	"testing"
)

func TestNoiseHandshake(t *testing.T) {
	kp1, err := CipherSuite.GenerateKeypair(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	kp2, err := CipherSuite.GenerateKeypair(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	ns1, err := NewNoiseState(kp1, kp2.Public)
	if err != nil {
		t.Fatal(err)
	}
	ns2, err := NewNoiseState(kp2, nil)
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
