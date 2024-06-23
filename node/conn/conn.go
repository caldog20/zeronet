package conn

import (
	"net"
	"time"
)

const (
	UDPType = "udp4"
)

type Conn struct {
	uc *net.UDPConn
}

func (conn *Conn) WriteToUDP(b []byte, addr *net.UDPAddr) (int, error) {
	n, err := conn.uc.WriteToUDP(b, addr)
	return n, err
}

func (conn *Conn) ReadFromUDP(b []byte) (int, *net.UDPAddr, error) {
	n, raddr, err := conn.uc.ReadFromUDP(b)
	return n, raddr, err
}

func (conn *Conn) LocalAddr() net.Addr {
	return conn.uc.LocalAddr()
}

func (conn *Conn) Close() error {
	return conn.uc.Close()
}

func (conn *Conn) SetReadDeadline(t time.Time) error {
	return conn.uc.SetReadDeadline(t)
}
