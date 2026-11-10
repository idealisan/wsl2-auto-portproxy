package proxy

import (
	"net"
	"testing"
)

func TestPortOccupiedByLoopbackListener(t *testing.T) {
	ln, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	if !portOccupied(port) {
		t.Fatalf(":%d has a loopback listener, expected occupied", port)
	}
}

func TestPortOccupiedByWildcardListener(t *testing.T) {
	ln, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	if !portOccupied(port) {
		t.Fatalf(":%d has a wildcard listener, expected occupied", port)
	}
}

func TestPortFreeWhenNothingListening(t *testing.T) {
	port := int(freePort(t))
	if portOccupied(port) {
		t.Fatalf(":%d is free, expected not occupied", port)
	}
}

func TestLocalProbeIPs(t *testing.T) {
	ips := localProbeIPs()
	seen := map[string]bool{}
	for _, ip := range ips {
		if ip.To4() == nil {
			t.Fatalf("probe list should contain IPv4 addresses only, got %s", ip)
		}
		if seen[ip.String()] {
			t.Fatalf("duplicate probe address %s", ip)
		}
		seen[ip.String()] = true
	}
	if !seen["127.0.0.1"] || !seen["0.0.0.0"] {
		t.Fatalf("probe list must include loopback and wildcard, got %v", seen)
	}
}
