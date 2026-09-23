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
// accept on :TargetPort, dial WslIp:AgentPort, send "TargetPort\n",
// then pipe both directions. Existing connections drain on Stop;
// only the listener is closed.
type Proxy struct {
	TargetPort int64
	WslIp      string
	AgentPort  int
	Listener   *net.TCPListener
	IsRunning  bool
}

func (p *Proxy) Start() error {
	localAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", p.TargetPort))
	if err != nil {
		log.Printf("resolve local addr error,%s\n", err)
		return err
	}
	p.Listener, err = net.ListenTCP("tcp", localAddr)
	if err != nil {
		log.Printf("Could not start proxy server on %d: %v\n", p.TargetPort, err)
		return err
	}
	log.Printf("new proxy :%d -> %s:%d/%d", p.TargetPort, p.WslIp, p.AgentPort, p.TargetPort)
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
	log.Printf("proxy stop, :%d -> %s:%d/%d", p.TargetPort, p.WslIp, p.AgentPort, p.TargetPort)
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
