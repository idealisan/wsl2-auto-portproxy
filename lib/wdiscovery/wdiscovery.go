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

// Target is one Windows listen port and where to forward it. ID uniquely
// identifies the (agent, port) pair, so several WSL distros can listen on the
// same port without overwriting each other.
type Target struct {
	ID        string
	Port      int64
	WslIP     string
	AgentPort int
	Proc      string
}

type source struct {
	ip        string
	ports     []int64
	agentPort int
	procs     map[int64]string
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
		payload := string(buf[:n])
		ports, agentPort, err := broadcast.Parse(payload)
		if err != nil {
			log.Printf("bad broadcast from %s: %s", udpAddr.IP.String(), err)
			continue
		}
		src := udpAddr.IP.String()
		// WSL2 distros can share one IP, so the agent's random channel port
		// tells them apart. A restarted agent gets a new key and the old one
		// expires after Expiry.
		key := src + "/" + strconv.Itoa(agentPort)
		mu.Lock()
		sources[key] = &source{
			ip:        src,
			ports:     ports,
			agentPort: agentPort,
			procs:     broadcast.ParseProcs(payload),
			lastSeen:  time.Now(),
		}
		mu.Unlock()
		log.Printf("discover %s: ports=%v agentPort=%d", key, ports, agentPort)
	}
}

// Snapshot returns the current desired forwards, purging expired sources.
// Every (agent, port) pair is returned: when two agents share a port, the
// proxy's bind fallback gives the second one a free Windows port.
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
	var out []Target
	for _, k := range keys {
		s := sources[k]
		for _, p := range s.ports {
			out = append(out, Target{
				ID:        k + "/" + strconv.FormatInt(p, 10),
				Port:      p,
				WslIP:     s.ip,
				AgentPort: s.agentPort,
				Proc:      s.procs[p],
			})
		}
	}
	return out
}
