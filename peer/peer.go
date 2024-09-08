package peer

import (
	"encoding/base64"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/flynn/noise"

	"github.com/caldog20/zeronet/pkg/header"
)

type Peer struct {
	ID      uint32
	keypair noise.DHKey
	rs      []byte

	inbound  chan []byte
	outbound chan []byte

	ns      *NoiseState
	wg      sync.WaitGroup
	running atomic.Bool
	ready   sync.RWMutex
}

func NewPeer(id uint32, kp noise.DHKey, rs string) (*Peer, error) {
	rsSlice, err := base64.StdEncoding.DecodeString(rs)
	if err != nil {
		return nil, err
	}

	return &Peer{
		ID:       id,
		keypair:  kp,
		rs:       rsSlice,
		inbound:  make(chan []byte, 2),
		outbound: make(chan []byte, 2),
	}, nil
}

func (p *Peer) Start() {
	p.wg.Add(2)
	go p.handleInbound()
	go p.handleOutbound()
	p.running.Store(true)
}

func (p *Peer) Stop() {
	p.running.Store(false)
	p.inbound <- nil
	p.outbound <- nil
	// Wait here for peer routines to stop
	p.wg.Wait()
}

func (p *Peer) handleInbound() {
	for b := range p.inbound {
		if b == nil {
			return
		}
		p.ready.RLock()
		//p.handleInboundDataPacket(h, b[header.HeaderLen:])
		p.ready.RUnlock()
	}
}

func (p *Peer) handleOutbound() {
	for b := range p.outbound {
		if b == nil {
			return
		}
		p.ready.RLock()

		h := header.NewHeader()
		packet := make([]byte, 1400)
		encoded, err := h.Encode(packet, header.Data, p.ID, p.ns.nonce.Load())
		if err != nil {
			fmt.Printf("error encoding header for outbound packet peer %d\n", p.ID)
			p.ready.Unlock()
			continue
		}
		_ = encoded
		p.ready.RUnlock()
	}
}

func (p *Peer) flushChannels() {
	for len(p.inbound) > 0 {
		<-p.inbound
	}

	for len(p.outbound) > 0 {
		<-p.outbound
	}
}
