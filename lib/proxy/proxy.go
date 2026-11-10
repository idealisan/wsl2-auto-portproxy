package proxy

import (
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"time"
)

// dialTimeout bounds how long a client waits for the agent channel.
const dialTimeout = 5 * time.Second

// keepAlivePeriod detects half-open connections on both legs.
const keepAlivePeriod = 15 * time.Second

// Proxy forwards one Windows listen port through the agent channel:
// accept on :ListenPort, dial WslIp:AgentPort, send "TargetPort\n",
// then pipe both directions.
//
// By default the listener binds 127.0.0.1, so the forward is reachable only
// from the Windows host itself. Setting External binds 0.0.0.0 instead, which
// exposes it to the LAN; the GUI offers this as an opt-in checkbox.
//
// ListenPort normally equals TargetPort, but when that port is already bound
// on any local address Start asks the OS for a free ephemeral port instead,
// so a conflict never disables the forward. The GUI shows the actual mapping.
// Existing connections drain on Stop; only the listener is closed.
type Proxy struct {
	ID         string // (agent, port) identity, stable across reconciles
	TargetPort int64  // port inside WSL, sent to the agent
	ListenPort int64  // port bound on Windows (0 -> start with TargetPort)
	WslIp      string
	AgentPort  int
	External   bool // bind 0.0.0.0 instead of 127.0.0.1
	Listener   *net.TCPListener
	IsRunning  bool
}

// bindIP is the address the proxy listens on. Loopback keeps a forward
// private to the host; the wildcard address is only used when the user opted
// into forwarding to an external (LAN-reachable) port.
func (p *Proxy) bindIP() net.IP {
	if p.External {
		return net.IPv4zero
	}
	return net.IPv4(127, 0, 0, 1)
}

func (p *Proxy) Start() error {
	host := p.bindIP()

	// A port is reused only when it is completely free on every local
	// address. Any occupancy at all means a random port, so the same number
	// never has two different owners on the host.
	random := false
	if p.ListenPort == 0 {
		p.ListenPort = p.TargetPort
		if portOccupied(int(p.TargetPort)) {
			random = true
		}
	}
	want := int(p.ListenPort)
	if random {
		want = 0
	}

	ln, err := net.ListenTCP("tcp", &net.TCPAddr{IP: host, Port: want})
	if err != nil {
		// The requested port became unavailable (or is held on another local
		// address); the OS picks a free one on the same bind address.
		ln, err = net.ListenTCP("tcp", &net.TCPAddr{IP: host, Port: 0})
		if err != nil {
			log.Printf("[%s] could not start proxy for WSL :%d: %v", p.ID, p.TargetPort, err)
			return err
		}
		random = true
	}
	p.ListenPort = int64(ln.Addr().(*net.TCPAddr).Port)
	p.Listener = ln
	if random {
		log.Printf("[%s] WSL :%d is in use on this host, forwarding on :%d",
			p.ID, p.TargetPort, p.ListenPort)
	}
	log.Printf("[%s] new proxy %s:%d -> %s:%d/%d", p.ID, host, p.ListenPort,
		p.WslIp, p.AgentPort, p.TargetPort)

	go func() {
		for {
			conn, err := p.Listener.AcceptTCP()
			if err != nil {
				if p.IsRunning {
					log.Println("Could not accept client connection:", err)
				}
				break
			}
			go p.handleConn(conn)
		}
	}()
	p.IsRunning = true
	return nil
}

func (p *Proxy) Stop() error {
	p.IsRunning = false
	log.Printf("[%s] proxy stop, :%d -> %s:%d/%d", p.ID, p.ListenPort, p.WslIp, p.AgentPort, p.TargetPort)
	if p.Listener == nil {
		return nil
	}
	return p.Listener.Close()
}

func (p *Proxy) handleConn(client *net.TCPConn) {
	defer client.Close()
	_ = client.SetKeepAlive(true)
	_ = client.SetKeepAlivePeriod(keepAlivePeriod)

	agent, err := net.DialTimeout("tcp", net.JoinHostPort(p.WslIp, strconv.Itoa(p.AgentPort)), dialTimeout)
	if err != nil {
		log.Printf("dial agent %s:%d: %s", p.WslIp, p.AgentPort, err)
		return
	}
	channel, ok := agent.(*net.TCPConn)
	if !ok {
		agent.Close()
		return
	}
	defer channel.Close()
	_ = channel.SetKeepAlive(true)
	_ = channel.SetKeepAlivePeriod(keepAlivePeriod)

	_ = channel.SetDeadline(time.Now().Add(dialTimeout))
	if _, err := fmt.Fprintf(channel, "%d\n", p.TargetPort); err != nil {
		log.Printf("send target port: %s", err)
		return
	}
	_ = channel.SetDeadline(time.Time{})

	log.Printf("client '%v' -> agent %s:%d/%d\n", client.RemoteAddr(), p.WslIp, p.AgentPort, p.TargetPort)
	pipe(client, channel)
}

func pipe(a, b *net.TCPConn) {
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(b, a)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(a, b)
		done <- struct{}{}
	}()
	<-done
	_ = a.Close()
	_ = b.Close()
}
