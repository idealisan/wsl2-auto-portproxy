// wslpp-gui is the Windows GUI front end for wslpp. It lists the TCP ports
// listening inside WSL together with their owning process names and lets the
// user start or stop forwarding per row, or for a multi-selection. Unlike the
// headless wslpp.exe it forwards nothing until the user asks for it.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/HobaiRiku/wsl2-auto-portproxy/lib/proxy"
	"github.com/HobaiRiku/wsl2-auto-portproxy/lib/service"
	"github.com/HobaiRiku/wsl2-auto-portproxy/lib/storage"
	"github.com/HobaiRiku/wsl2-auto-portproxy/lib/wdiscovery"
)

var version string

func main() {
	mygo.App.SetName("wslpp-gui")
	a := newApp()

	mygo.App.WhenReady(func() {
		a.mu.Lock()
		a.win = mygo.NewWindow(mygo.WindowOptions{
			Title:     "wslpp 端口转发",
			Width:     760,
			Height:    480,
			MinWidth:  560,
			MinHeight: 320,
			Content:   ui.View(a.view),
		})
		a.mu.Unlock()

		go wdiscovery.ListenLoop()
		go a.reconcileLoop()
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func (a *guiApp) reconcileLoop() {
	for {
		a.reconcile()
		time.Sleep(time.Second)
	}
}

// reconcile mirrors the headless main loop, but forwards only the ports the
// user enabled. It is the sole writer of storage.ProxyPool in this program.
func (a *guiApp) reconcile() {
	desired := wdiscovery.Snapshot()

	used := map[int64]bool{}
	var errs []string
	windowsPorts, err := service.GetLocalUsedPorts()
	if err != nil {
		errs = append(errs, "检测本机端口失败: "+err.Error())
	}
	for _, p := range windowsPorts {
		used[p] = true
	}

	a.mu.Lock()
	enabled := make(map[int64]bool, len(a.enabled))
	for p := range a.enabled {
		enabled[p] = true
	}
	a.mu.Unlock()

	should := make([]wdiscovery.Target, 0, len(desired))
	for _, t := range desired {
		if used[t.Port] || !enabled[t.Port] {
			continue
		}
		should = append(should, t)
	}

	storage.PoolMu.Lock()
	for _, t := range should {
		matched := false
		for i := range storage.ProxyPool {
			p := &storage.ProxyPool[i]
			if p.TargetPort != t.Port {
				continue
			}
			matched = true
			if p.WslIp != t.WslIP || p.AgentPort != t.AgentPort {
				// Stale backend (agent restarted / WSL IP drifted); stop it,
				// the cleanup below drops it and it is recreated next round.
				_ = p.Stop()
			} else if !p.IsRunning {
				if err := p.Start(); err != nil {
					errs = append(errs, fmt.Sprintf("启动 :%d 失败: %s", t.Port, err))
				}
			}
			break
		}
		if !matched {
			np := proxy.Proxy{TargetPort: t.Port, WslIp: t.WslIP, AgentPort: t.AgentPort}
			if err := np.Start(); err != nil {
				errs = append(errs, fmt.Sprintf("启动 :%d 失败: %s", t.Port, err))
			} else {
				storage.ProxyPool = append(storage.ProxyPool, np)
			}
		}
	}

	for i := 0; i < len(storage.ProxyPool); {
		keep := false
		for _, t := range should {
			pp := &storage.ProxyPool[i]
			if pp.TargetPort == t.Port && pp.WslIp == t.WslIP && pp.AgentPort == t.AgentPort {
				keep = true
				break
			}
		}
		if !keep {
			_ = storage.ProxyPool[i].Stop()
		}
		if !storage.ProxyPool[i].IsRunning {
			storage.ProxyPool = append(storage.ProxyPool[:i], storage.ProxyPool[i+1:]...)
		} else {
			i++
		}
	}

	running := map[int64]bool{}
	for i := range storage.ProxyPool {
		if storage.ProxyPool[i].IsRunning {
			running[storage.ProxyPool[i].TargetPort] = true
		}
	}
	storage.PoolMu.Unlock()

	online := len(desired) > 0
	wslIP := ""
	if online {
		wslIP = desired[0].WslIP
	}
	a.applyState(desired, used, running, online, wslIP, errs)
}
