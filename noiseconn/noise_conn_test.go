package noiseconn

import (
	"context"
	"crypto/rand"
	"log"
	"net"
	"os"
	"sync"
	"testing"

	"github.com/flynn/noise"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
)

var (
	kp1, kp2     noise.DHKey
	nc1, nc2     *NoiseConn
	conn1, conn2 net.Conn
)

func TestMain(m *testing.M) {
	cs := noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2s)
	var err error
	kp1, err = cs.GenerateKeypair(rand.Reader)
	if err != nil {
		log.Fatal(err)
	}

	kp2, err = cs.GenerateKeypair(rand.Reader)
	if err != nil {
		log.Fatal(err)
	}

	conn1, conn2 = net.Pipe()

	nc1 = NewNoiseConn(kp1, kp2.Public)
	nc2 = NewNoiseConn(kp2, kp1.Public)

	os.Exit(m.Run())
}

func TestNoiseConnHandshake(t *testing.T) {
	eg, egCtx := errgroup.WithContext(context.Background())
	eg.Go(func() error {
		nc2.SetConn(conn2)
		return nc2.Accept()
	})
	eg.Go(func() error {
		nc1.SetConn(conn1)
		return nc1.Dial()
	})

	if err := eg.Wait(); err != nil {
		log.Fatal(err)
	}
	_ = egCtx
}

func TestNoiseConnReadWrite(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()

		for range 3 {
			p := make([]byte, 1400)
			n, err := nc2.Read(p)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("received %d bytes from peer: %s", n, string(p))
		}
	}()

	go func() {
		defer wg.Done()

		for range 3 {
			p := []byte("Hello")
			n, err := nc1.Write(p)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("wrote %d bytes to peer: %s", n, p)
		}
	}()

	wg.Wait()
}

func TestNoiseConnDialAcceptError(t *testing.T) {
	uc, _ := net.Pipe()
	conn := NewNoiseConn(kp1, kp2.Public)
	conn.SetConn(uc)
	var err error
	done := make(chan struct{})
	go func() {
		err = conn.Accept()
		done <- struct{}{}
	}()

	// time.Sleep(time.Second * 2)
	go func() {
		err = conn.Dial()
		done <- struct{}{}
	}()

	<-done
	assert.EqualError(
		t,
		err,
		"noiseconn must be idle before attempting to call accept: current state: Dialing",
		"unexpected state returned in error when calling dial after accept",
	)
}
