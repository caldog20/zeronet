package node

import (
	"errors"
	"log"
	"net"
	"runtime"
)

type OnUDPPacket func(buffer *InboundBuffer, index int)

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
