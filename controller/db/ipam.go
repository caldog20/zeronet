package db

import (
	"errors"
	"log"
	"net/netip"

	"go4.org/netipx"

	"github.com/caldog20/zeronet/controller/types"
)

func (s *Store) GetAllocatedIPs() ([]netip.Addr, error) {
	var ips []string
	err := s.db.Model(&types.Peer{}).Pluck("ip", &ips).Error
	if err != nil {
		return nil, err
	}

	var allocatedIPs []netip.Addr
	// Convert IP strings into netip.Addr
	for _, ip := range ips {
		parsedIP, err := netip.ParseAddr(ip)
		if err != nil {
			log.Printf("error parsing IP %s\n", ip)
			continue
		}
		allocatedIPs = append(allocatedIPs, parsedIP)
	}
	return allocatedIPs, nil
}

func (s *Store) AllocatePeerIP(prefix netip.Prefix) (string, error) {
	allocatedIPs, err := s.GetAllocatedIPs()
	if err != nil {
		return "", errors.New("error retrieving allocated IPs from store")
	}

	var builder netipx.IPSetBuilder

	var net, bcast netip.Addr
	ipRange := netipx.RangeOfPrefix(prefix)
	net = ipRange.From()
	bcast = ipRange.To()

	builder.Add(net)
	builder.Add(bcast)

	for _, allocatedIP := range allocatedIPs {
		builder.Add(allocatedIP)
	}

	ipSet, err := builder.IPSet()
	if err != nil {
		return "", errors.New("error building IP set")
	}

	addr := prefix.Addr()
	// Loop through and allocate next free IP address
	for {
		if ipSet.Contains(addr) {
			addr = addr.Next()
			continue
		}
		break
	}

	return addr.String(), nil
}
