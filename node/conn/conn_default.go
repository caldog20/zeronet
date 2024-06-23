//go:build !unix

package conn

import (
	"fmt"
	"net"
)

func NewConn(port uint16) (*Conn, error) {

	laddr, err := net.ResolveUDPAddr(UDPType, fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return nil, err
	}

	udpconn, err := net.ListenUDP(UDPType, laddr)
	if err != nil {
		return nil, err
	}

	conn := &Conn{uc: udpconn}

	return conn, nil
}
