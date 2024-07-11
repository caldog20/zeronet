package node

import (
	"errors"
	"log"
	"os"
	"runtime"
)

type OnTunnelPacket func(buffer *OutboundBuffer)

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
