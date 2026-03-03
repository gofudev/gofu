package http

import (
	"context"
	"fmt"
	"net"
)

var blockedNetworks = func() []*net.IPNet {
	cidrs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"fc00::/7",
		"fe80::/10",
		"::1/128",
	}
	nets := make([]*net.IPNet, len(cidrs))
	for i, cidr := range cidrs {
		_, n, _ := net.ParseCIDR(cidr)
		nets[i] = n
	}
	return nets
}()

func isBlockedIP(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
	}
	for _, n := range blockedNetworks {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

type resolver interface {
	LookupIPAddr(host string) ([]net.IP, error)
}

type safeDialer struct {
	resolver resolver
}

func (d *safeDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	ips, err := d.resolver.LookupIPAddr(host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no addresses found for %s", host)
	}

	ip := ips[0]
	if isBlockedIP(ip) {
		return nil, fmt.Errorf("request to blocked address %s", ip)
	}

	pinnedAddr := net.JoinHostPort(ip.String(), port)
	return (&net.Dialer{}).DialContext(ctx, network, pinnedAddr)
}
