package node

import (
	"errors"
	"log"
	"net"
	"os"
	"runtime"
)

type OnUDPPacket func(buffer *InboundBuffer, index int)
type OnTunnelPacket func(buffer *OutboundBuffer)

func (node *Node) ReadUDPPackets(callback OnUDPPacket, index int) {
	runtime.LockOSThread()
	for {
		buffer := GetInboundBuffer()
		n, raddr, err := node.conn.ReadFromUDP(buffer.in)
		if err != nil {
			PutInboundBuffer(buffer)
			if errors.Is(err, net.ErrClosed) {
				return
			}
			log.Printf("%v", err)
			continue
		}

		buffer.size = n
		buffer.raddr = raddr
		callback(buffer, index)
	}
}

func (node *Node) ReadTunPackets(callback OnTunnelPacket) {
	runtime.LockOSThread()
	for {
		buffer := GetOutboundBuffer()
		n, err := node.tun.Read(buffer.packet)
		if err != nil {
			PutOutboundBuffer(buffer)
			if errors.Is(err, os.ErrClosed) {
				return
			}
			log.Printf("%v", err)
			continue
		}

		buffer.size = n
		callback(buffer)
	}
}
