// wslpp-agent runs inside WSL: it scans all local TCP listening ports,
// broadcasts them on UDP 1033, and serves the single random-port TCP
// channel that Windows forwards through.
package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/HobaiRiku/wsl2-auto-portproxy/lib/broadcast"
	"github.com/HobaiRiku/wsl2-auto-portproxy/lib/discover"
)

const (
	broadcastInterval = 3 * time.Second
	handshakeTimeout  = 5 * time.Second
	dialTimeout       = 5 * time.Second
	keepAlivePeriod   = 15 * time.Second
)

func main() {
	// logs go to stdout; stderr stays reserved for real process errors
	log.SetOutput(os.Stdout)
	ln, err := net.ListenTCP("tcp", &net.TCPAddr{Port: 0})
	if err != nil {
		log.Fatalf("listen channel: %s", err)
	}
	agentPort := ln.Addr().(*net.TCPAddr).Port
	log.Printf("agent channel listening on :%d", agentPort)

	pc, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		log.Fatalf("udp socket: %s", err)
	}
	defer pc.Close()
	if err := broadcast.SetBroadcast(pc); err != nil {
		log.Printf("warn: SO_BROADCAST not set (%s), subnet broadcasts may fail; host unicast still works", err)
	}

	go acceptLoop(ln)

	for {
		announce(pc, agentPort)
		time.Sleep(broadcastInterval)
	}
}

func announce(pc net.PacketConn, agentPort int) {
	exclude := map[int]bool{agentPort: true}
	ports := discover.LocalTCPPorts(exclude)
	payload := broadcast.Build(ports, agentPort)
	for _, dst := range broadcast.Targets() {
		if _, err := pc.WriteTo([]byte(payload), dst); err != nil {
			log.Printf("broadcast to %s failed: %s", dst.String(), err)
		}
	}
}

func acceptLoop(ln *net.TCPListener) {
	for {
		c, err := ln.AcceptTCP()
		if err != nil {
			log.Printf("accept: %s", err)
			continue
		}
		go handleChannel(c)
	}
}

// handleChannel reads one "PORT\n" handshake, dials that port on loopback,
// and pipes both directions. Any failure just closes the connection;
// the Windows side then closes its client connection in turn.
func handleChannel(c *net.TCPConn) {
	defer c.Close()
	_ = c.SetKeepAlive(true)
	_ = c.SetKeepAlivePeriod(keepAlivePeriod)

	_ = c.SetDeadline(time.Now().Add(handshakeTimeout))
	line, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		return
	}
	n, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || n <= 0 || n > 65535 {
		return
	}
	_ = c.SetDeadline(time.Time{})

	backend, err := dialLoopback(n)
	if err != nil {
		log.Printf("dial 127.0.0.1:%d: %s", n, err)
		return
	}
	defer backend.Close()
	pipe(c, backend)
	log.Printf("channel %s -> 127.0.0.1:%d closed", c.RemoteAddr().String(), n)
}

func dialLoopback(port int) (*net.TCPConn, error) {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), dialTimeout)
	if err == nil {
		if tc, ok := conn.(*net.TCPConn); ok {
			_ = tc.SetKeepAlive(true)
			_ = tc.SetKeepAlivePeriod(keepAlivePeriod)
			return tc, nil
		}
		conn.Close()
	}
	conn6, err6 := net.DialTimeout("tcp", fmt.Sprintf("::1:%d", port), dialTimeout)
	if err6 != nil {
		if err != nil {
			return nil, err
		}
		return nil, err6
	}
	if tc, ok := conn6.(*net.TCPConn); ok {
		_ = tc.SetKeepAlive(true)
		_ = tc.SetKeepAlivePeriod(keepAlivePeriod)
		return tc, nil
	}
	conn6.Close()
	return nil, fmt.Errorf("unexpected conn type")
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
