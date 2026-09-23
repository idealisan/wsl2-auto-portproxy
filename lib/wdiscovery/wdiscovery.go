// Package wdiscovery listens for agent UDP broadcasts on :1033 and keeps
// the desired forward set: which ports, on which WSL IP, via which agent port.
package wdiscovery

import (
	"log"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/HobaiRiku/wsl2-auto-portproxy/lib/broadcast"
)

// Expiry drops a source that stopped broadcasting (3 missed 3s rounds).
const Expiry = 9 * time.Second

// Target is one Windows listen port and where to forward it.
type Target struct {
	Port      int64
	WslIP     string
	AgentPort int
}

type source struct {
	ports     []int64
	agentPort int
	lastSeen  time.Time
}

var (
	mu      sync.Mutex
	sources = map[string]*source{}
)

// ListenLoop binds UDP :1033 and applies every broadcast. It retries forever.
func ListenLoop() {
	for {
		pc, err := net.ListenPacket("udp4", ":"+strconv.Itoa(broadcast.Port))
		if err != nil {
			log.Printf("listen udp :%d: %s, retrying", broadcast.Port, err)
			time.Sleep(3 * time.Second)
			continue
		}
		serve(pc)
		pc.Close()
	}
}

func serve(pc net.PacketConn) {
	buf := make([]byte, 65535)
	for {
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			log.Printf("udp read: %s", err)
			return
		}
		udpAddr, ok := addr.(*net.UDPAddr)
		if !ok {
			continue
		}
		ports, agentPort, err := broadcast.Parse(string(buf[:n]))
		if err != nil {
			log.Printf("bad broadcast from %s: %s", udpAddr.IP.String(), err)
			continue
		}
		src := udpAddr.IP.String()
		mu.Lock()
		sources[src] = &source{ports: ports, agentPort: agentPort, lastSeen: time.Now()}
		mu.Unlock()
		log.Printf("discover %s: ports=%v agentPort=%d", src, ports, agentPort)
	}
}

// Snapshot returns the current desired forwards, purging expired sources.
// One port is forwarded once: first source (by sorted IP) wins.
func Snapshot() []Target {
	mu.Lock()
	defer mu.Unlock()
	now := time.Now()
	for k, s := range sources {
		if now.Sub(s.lastSeen) > Expiry {
			log.Printf("source %s expired", k)
			delete(sources, k)
		}
	}
	keys := make([]string, 0, len(sources))
	for k := range sources {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	seen := map[int64]bool{}
	var out []Target
	for _, k := range keys {
		s := sources[k]
		for _, p := range s.ports {
			if seen[p] {
				continue
			}
			seen[p] = true
			out = append(out, Target{Port: p, WslIP: k, AgentPort: s.agentPort})
		}
	}
	return out
}
