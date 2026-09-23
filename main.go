package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/HobaiRiku/wsl2-auto-portproxy/lib/proxy"
	"github.com/HobaiRiku/wsl2-auto-portproxy/lib/service"
	"github.com/HobaiRiku/wsl2-auto-portproxy/lib/storage"
	"github.com/HobaiRiku/wsl2-auto-portproxy/lib/wdiscovery"
)

var version string

func main() {
	// logs go to stdout; stderr stays reserved for real process errors
	log.SetOutput(os.Stdout)
	// print version
	var showVersion bool
	flag.BoolVar(&showVersion, "v", false, "show version")
	flag.Parse()
	if showVersion {
		fmt.Println(version)
		os.Exit(1)
	}

	// learn {wslIp, ports, agentPort} from agent UDP broadcasts
	go wdiscovery.ListenLoop()

	for {
		desired := wdiscovery.Snapshot()

		// ports already used locally are omitted
		used := map[int64]bool{}
		windowsPorts, err := service.GetLocalUsedPorts()
		if err != nil {
			log.Println(err)
		}
		for _, p := range windowsPorts {
			used[p] = true
		}

		// create proxy
		for _, t := range desired {
			if used[t.Port] {
				continue
			}
			matched := false
			for i := range storage.ProxyPool {
				p := &storage.ProxyPool[i]
				if p.TargetPort != t.Port {
					continue
				}
				matched = true
				if p.WslIp != t.WslIP || p.AgentPort != t.AgentPort {
					// stale backend (agent restarted / WSL IP drifted);
					// stop it, the cleanup below drops it and it is recreated next round
					_ = p.Stop()
				} else if !p.IsRunning {
					if err := p.Start(); err != nil {
						log.Printf("start proxy :%d error,%s\n", t.Port, err)
					}
				}
				break
			}
			if !matched {
				np := proxy.Proxy{TargetPort: t.Port, WslIp: t.WslIP, AgentPort: t.AgentPort}
				if err := np.Start(); err != nil {
					log.Printf("start proxy :%d error,%s\n", t.Port, err)
				} else {
					storage.ProxyPool = append(storage.ProxyPool, np)
				}
			}
		}

		// check for delete update
		for i := 0; i < len(storage.ProxyPool); {
			keep := false
			for _, t := range desired {
				pp := &storage.ProxyPool[i]
				if pp.TargetPort == t.Port && pp.WslIp == t.WslIP && pp.AgentPort == t.AgentPort {
					keep = true
					break
				}
			}
			if !keep {
				_ = storage.ProxyPool[i].Stop()
			}
			// delete
			if !storage.ProxyPool[i].IsRunning {
				storage.ProxyPool = append(storage.ProxyPool[:i], storage.ProxyPool[i+1:]...)
			} else {
				i++
			}
		}
		time.Sleep(time.Second * 1)
	}
}
