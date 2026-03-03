package http

import (
	"context"
	"net"
	"testing"
)

func TestIsBlockedIP(t *testing.T) {
	blocked := []struct {
		name string
		ip   string
	}{
		{"private 10/8", "10.0.0.1"},
		{"private 10/8 high", "10.255.255.255"},
		{"private 172.16/12", "172.16.0.1"},
		{"private 172.31/12", "172.31.255.255"},
		{"private 192.168/16", "192.168.1.1"},
		{"loopback", "127.0.0.1"},
		{"loopback high", "127.255.255.255"},
		{"link-local", "169.254.1.1"},
		{"cloud metadata", "169.254.169.254"},
		{"ipv6 loopback", "::1"},
		{"ipv6 link-local", "fe80::1"},
		{"ipv6 unique-local", "fd00::1"},
		{"ipv4-mapped private", "::ffff:10.0.0.1"},
		{"ipv4-mapped loopback", "::ffff:127.0.0.1"},
		{"ipv4-mapped 192.168", "::ffff:192.168.1.1"},
	}

	for _, tc := range blocked {
		ip := net.ParseIP(tc.ip)
		if ip == nil {
			t.Fatalf("failed to parse IP %s", tc.ip)
		}
		if !isBlockedIP(ip) {
			t.Errorf("%s (%s): expected blocked", tc.name, tc.ip)
		}
	}

	allowed := []struct {
		name string
		ip   string
	}{
		{"public ipv4", "8.8.8.8"},
		{"public ipv4 2", "1.1.1.1"},
		{"public ipv4 3", "203.0.113.1"},
		{"ipv4-mapped public", "::ffff:8.8.8.8"},
	}

	for _, tc := range allowed {
		ip := net.ParseIP(tc.ip)
		if ip == nil {
			t.Fatalf("failed to parse IP %s", tc.ip)
		}
		if isBlockedIP(ip) {
			t.Errorf("%s (%s): expected allowed", tc.name, tc.ip)
		}
	}
}

type mockResolver struct {
	ips []net.IP
	err error
}

func (m *mockResolver) LookupIPAddr(host string) ([]net.IP, error) {
	return m.ips, m.err
}

func TestSafeDialerBlocksPrivateIP(t *testing.T) {
	d := &safeDialer{
		resolver: &mockResolver{ips: []net.IP{net.ParseIP("10.0.0.1")}},
	}
	_, err := d.DialContext(context.Background(), "tcp", "example.com:80")
	if err == nil {
		t.Fatal("expected error for private IP")
	}
}

func TestSafeDialerBlocksLoopback(t *testing.T) {
	d := &safeDialer{
		resolver: &mockResolver{ips: []net.IP{net.ParseIP("127.0.0.1")}},
	}
	_, err := d.DialContext(context.Background(), "tcp", "example.com:80")
	if err == nil {
		t.Fatal("expected error for loopback")
	}
}

func TestSafeDialerBlocksIPv6Loopback(t *testing.T) {
	d := &safeDialer{
		resolver: &mockResolver{ips: []net.IP{net.ParseIP("::1")}},
	}
	_, err := d.DialContext(context.Background(), "tcp", "example.com:80")
	if err == nil {
		t.Fatal("expected error for IPv6 loopback")
	}
}

func TestSafeDialerBlocksLinkLocal(t *testing.T) {
	d := &safeDialer{
		resolver: &mockResolver{ips: []net.IP{net.ParseIP("169.254.1.1")}},
	}
	_, err := d.DialContext(context.Background(), "tcp", "example.com:80")
	if err == nil {
		t.Fatal("expected error for link-local")
	}
}

func TestSafeDialerBlocksMetadata(t *testing.T) {
	d := &safeDialer{
		resolver: &mockResolver{ips: []net.IP{net.ParseIP("169.254.169.254")}},
	}
	_, err := d.DialContext(context.Background(), "tcp", "example.com:80")
	if err == nil {
		t.Fatal("expected error for metadata endpoint")
	}
}

func TestSafeDialerBlocksUniqueLocal(t *testing.T) {
	d := &safeDialer{
		resolver: &mockResolver{ips: []net.IP{net.ParseIP("fd00::1")}},
	}
	_, err := d.DialContext(context.Background(), "tcp", "example.com:80")
	if err == nil {
		t.Fatal("expected error for unique-local")
	}
}

func TestSafeDialerBlocksMappedPrivate(t *testing.T) {
	d := &safeDialer{
		resolver: &mockResolver{ips: []net.IP{net.ParseIP("::ffff:10.0.0.1")}},
	}
	_, err := d.DialContext(context.Background(), "tcp", "example.com:80")
	if err == nil {
		t.Fatal("expected error for IPv4-mapped private")
	}
}
