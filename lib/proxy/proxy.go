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
// then pipe both directions. ListenPort normally equals TargetPort, but when
// that port is already bound on the host Start picks a free ephemeral port
// instead, so a conflict never disables the forward. The GUI shows the
// actual mapping. Existing connections drain on Stop; only the listener is
// closed.
type Proxy struct {
	ID         string // (agent, port) identity, stable across reconciles
	TargetPort int64  // port inside WSL, sent to the agent
	ListenPort int64  // port bound on Windows (0 -> start with TargetPort)
	WslIp      string
	AgentPort  int
	Listener   *net.TCPListener
	IsRunning  bool
}

func (p *Proxy) Start() error {
	if p.ListenPort == 0 {
		p.ListenPort = p.TargetPort
	}
	ln, err := net.ListenTCP("tcp", &net.TCPAddr{Port: int(p.ListenPort)})
	if err != nil {
		// The wildcard bind itself is the availability check. Only when it
		// genuinely fails (another wildcard listener, e.g. a second agent on
		// the same port) do we ask the OS for a free port. A service bound to
		// 127.0.0.1:P — wslhost's localhostForwarding, for instance — does
		// not block 0.0.0.0:P, and LAN traffic to WindowsIP:P still reaches
		// this proxy, so that port stays as-is.
		requested := p.ListenPort
		ln, err = net.ListenTCP("tcp", &net.TCPAddr{Port: 0})
		if err != nil {
			log.Printf("[%s] could not start proxy for WSL :%d: %v", p.ID, p.TargetPort, err)
			return err
		}
		p.ListenPort = int64(ln.Addr().(*net.TCPAddr).Port)
		log.Printf("[%s] :%d busy, forwarding WSL :%d on :%d instead",
			p.ID, requested, p.TargetPort, p.ListenPort)
	}
	p.Listener = ln
	log.Printf("[%s] new proxy :%d -> %s:%d/%d", p.ID, p.ListenPort, p.WslIp, p.AgentPort, p.TargetPort)
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
