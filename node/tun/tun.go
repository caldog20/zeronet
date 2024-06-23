package tun

import "net/netip"

const MTU = 1300

type Tun interface {
	Read(b []byte) (int, error)
	Write(b []byte) (int, error)

	Name() string
	Close() error
	MTU() (int, error)

	ConfigureIPAddress(addr netip.Prefix) error
}
