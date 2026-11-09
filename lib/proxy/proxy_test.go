package proxy

import (
	"net"
	"testing"
)

func TestStartFallsBackWhenPortBusy(t *testing.T) {
	// Occupy a port, then ask the proxy to forward that same WSL port.
	busy, err := net.ListenTCP("tcp", &net.TCPAddr{Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	port := int64(busy.Addr().(*net.TCPAddr).Port)

	p := &Proxy{TargetPort: port, WslIp: "127.0.0.1", AgentPort: 1}
	if err := p.Start(); err != nil {
		t.Fatalf("Start: %s", err)
	}
	defer p.Stop()

	if p.ListenPort == port {
		t.Fatalf("expected a free ephemeral port, still on :%d", port)
	}
	if p.ListenPort <= 0 || p.ListenPort > 65535 {
		t.Fatalf("invalid listen port %d", p.ListenPort)
	}
	if p.TargetPort != port {
		t.Fatalf("target port changed: %d", p.TargetPort)
	}
	if !p.IsRunning {
		t.Fatal("proxy should be running")
	}
}

func TestStartUsesTargetPortWhenFree(t *testing.T) {
	probe, err := net.ListenTCP("tcp", &net.TCPAddr{Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	port := int64(probe.Addr().(*net.TCPAddr).Port)
	probe.Close()

	p := &Proxy{TargetPort: port, WslIp: "127.0.0.1", AgentPort: 1}
	if err := p.Start(); err != nil {
		t.Fatalf("Start: %s", err)
	}
	defer p.Stop()

	if p.ListenPort != port {
		t.Fatalf("expected :%d, got :%d", port, p.ListenPort)
	}
}
