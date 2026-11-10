// wslpp-gui is the GUI front end for wslpp. It lists the TCP ports listening
// inside every WSL environment together with their owning process names and
// lets the user start or stop forwarding per row, or for a selection. Unlike
// the headless wslpp.exe it forwards nothing until the user asks for it.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/HobaiRiku/wsl2-auto-portproxy/lib/proxy"
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
			Width:     920,
			Height:    540,
			MinWidth:  640,
			MinHeight: 360,
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
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		a.reconcile()
		select {
		case <-ticker.C:
		case <-a.wake:
		}
	}
}

// wakeReconcile asks the loop to reconcile again immediately, without waiting
// for the next one-second tick. It never blocks if a wake is already pending.
func (a *guiApp) wakeReconcile() {
	select {
	case a.wake <- struct{}{}:
	default:
	}
}

// reconcile mirrors the headless main loop, but forwards only the rows the
// user enabled. It is the sole writer of storage.ProxyPool in this program.
func (a *guiApp) reconcile() {
	desired := wdiscovery.Snapshot()
	var errs []string

	// WSL2 distros can share an IP, so count distinct agent channels.
	agents := map[string]bool{}
	for _, t := range desired {
		agents[fmt.Sprintf("%s/%d", t.WslIP, t.AgentPort)] = true
	}

	a.mu.Lock()
	enabled := make(map[string]bool, len(a.enabled))
	for id := range a.enabled {
		enabled[id] = true
	}
	external := a.external
	a.mu.Unlock()

	should := make([]wdiscovery.Target, 0, len(desired))
	for _, t := range desired {
		if !enabled[t.ID] {
			continue
		}
		should = append(should, t)
	}

	// Changing the bind mode means every running proxy must be torn down and
	// recreated on the new address. The cleanup phase drops the stopped ones;
	// wake again so the rebuild does not wait for a full tick.
	rebind := false

	storage.PoolMu.Lock()
	for _, t := range should {
		matched := false
		for i := range storage.ProxyPool {
			p := &storage.ProxyPool[i]
			if p.ID != t.ID {
				continue
			}
			matched = true
			if p.WslIp != t.WslIP || p.AgentPort != t.AgentPort || p.External != external {
				if p.External != external {
					rebind = true
				}
				_ = p.Stop()
			} else if !p.IsRunning {
				if err := p.Start(); err != nil {
					errs = append(errs, fmt.Sprintf("启动 :%d 失败: %s", t.Port, err))
				}
			}
			break
		}
		if !matched {
			np := proxy.Proxy{ID: t.ID, TargetPort: t.Port, WslIp: t.WslIP,
				AgentPort: t.AgentPort, External: external}
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
			if pp.ID == t.ID && pp.WslIp == t.WslIP && pp.AgentPort == t.AgentPort && pp.External == external {
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

	running := map[string]int64{} // ID -> actual Windows listen port
	for i := range storage.ProxyPool {
		if storage.ProxyPool[i].IsRunning {
			running[storage.ProxyPool[i].ID] = storage.ProxyPool[i].ListenPort
		}
	}
	storage.PoolMu.Unlock()

	online := len(desired) > 0
	wslIP := ""
	if online {
		wslIP = desired[0].WslIP
	}
	a.applyState(desired, running, online, wslIP, len(agents), errs)
	if rebind {
		a.wakeReconcile()
	}
}
