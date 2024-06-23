//go:build unix

package conn

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

func NewConn(port uint16) (*Conn, error) {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var opErr error
			err := c.Control(func(fd uintptr) {
				opErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
			})
			if err != nil {
				return err
			}
			return opErr
		},
	}

	lp, err := lc.ListenPacket(context.Background(), UDPType, fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return nil, err
	}

	udpconn, ok := lp.(*net.UDPConn)
	if !ok {
		return nil, errors.New("error casting ListenPacket into UDP Conn")
	}

	conn := &Conn{
		uc: udpconn,
	}

	return conn, nil
}
