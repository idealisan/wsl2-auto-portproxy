package main

import (
	"cmp"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/HobaiRiku/wsl2-auto-portproxy/lib/wdiscovery"
)

// rowView is an immutable snapshot of one row, taken on the UI thread before
// any control is built. The list is virtualized, so per-row state must live
// in the model rather than in the cell callback.
type rowView struct {
	Port       int64 // port listening inside WSL
	ListenPort int64 // port actually bound on Windows (0 when not forwarding)
	Proc       string
	WSLIP      string
	AgentPort  int
	Checked    bool
	Enabled    bool
	Running    bool
}

type snapshot struct {
	rows      []rowView
	online    bool
	wslIP     string
	forwarded int
	message   string
	version   string
}

type guiApp struct {
	mu        sync.Mutex
	win       *mygo.Window
	listState *ui.ListState

	targets map[int64]wdiscovery.Target // latest broadcast, keyed by port
	checked map[int64]bool              // multi-selection
	enabled map[int64]bool              // ports the user asked to forward
	running map[int64]int64             // WSL port -> actual Windows listen port
	online  bool
	wslIP   string
	message string

	// sort is written by the table header on the UI thread and read by
	// snapshotLocked, which also runs on the UI thread, so it needs no lock.
	sort       ui.SortOrder
	flashUntil time.Time
}

func newApp() *guiApp {
	return &guiApp{
		listState: &ui.ListState{},
		targets:   map[int64]wdiscovery.Target{},
		checked:   map[int64]bool{},
		enabled:   map[int64]bool{},
		running:   map[int64]int64{},
		message:   "等待 WSL agent 广播…（请确认 wslpp-agent 正在 WSL 内运行）",
	}
}

// refresh asks the window for a new frame. Update runs the (empty) function on
// the main thread and then rebuilds the view, so the view always reads the
// freshest model state.
func (a *guiApp) refresh() {
	a.mu.Lock()
	win := a.win
	a.mu.Unlock()
	if win == nil {
		return
	}
	win.Update(func() {})
}

// snapshotLocked must be called with a.mu held. It orders the rows as the
// table's Sort says; the table shows rows in exactly the order its cell
// callback receives them.
func (a *guiApp) snapshotLocked() snapshot {
	ports := make([]int64, 0, len(a.targets))
	for p := range a.targets {
		ports = append(ports, p)
	}
	sort.Slice(ports, func(i, j int) bool { return ports[i] < ports[j] })

	rows := make([]rowView, 0, len(ports))
	forwarded := 0
	for _, p := range ports {
		t := a.targets[p]
		r := rowView{
			Port:       p,
			ListenPort: a.running[p],
			Proc:       t.Proc,
			WSLIP:      t.WslIP,
			AgentPort:  t.AgentPort,
			Checked:    a.checked[p],
			Enabled:    a.enabled[p],
			Running:    a.running[p] != 0,
		}
		if r.Running {
			forwarded++
		}
		rows = append(rows, r)
	}
	sortRows(rows, a.sort)
	return snapshot{
		rows:      rows,
		online:    a.online,
		wslIP:     a.wslIP,
		forwarded: forwarded,
		message:   a.message,
		version:   version,
	}
}

// applyState publishes the result of a reconcile pass and refreshes the view.
func (a *guiApp) applyState(desired []wdiscovery.Target, running map[int64]int64,
	online bool, wslIP string, errs []string) {
	a.mu.Lock()
	a.targets = make(map[int64]wdiscovery.Target, len(desired))
	for _, t := range desired {
		a.targets[t.Port] = t
	}
	a.running = running
	a.online = online
	a.wslIP = wslIP
	// Selections for ports that disappeared are dropped; enabled stays sticky
	// so a port that comes back is forwarded again without another click.
	for p := range a.checked {
		if _, ok := a.targets[p]; !ok {
			delete(a.checked, p)
		}
	}
	switch {
	case len(errs) > 0:
		a.message = strings.Join(errs, "；")
	case time.Now().Before(a.flashUntil):
		// Keep the short-lived message set by copyPort for a few seconds.
	case !online:
		a.message = "等待 WSL agent 广播…（请确认 wslpp-agent 正在 WSL 内运行）"
	default:
		a.message = fmt.Sprintf("发现 %d 个监听端口，已转发 %d 个；点列头排序、拖列头边缘调宽度、点端口数字复制",
			len(desired), len(running))
	}
	a.mu.Unlock()
	a.refresh()
}

func (a *guiApp) setChecked(port int64, on bool) {
	a.mu.Lock()
	if on {
		a.checked[port] = true
	} else {
		delete(a.checked, port)
	}
	a.mu.Unlock()
	a.refresh()
}

func (a *guiApp) toggle(port int64) {
	a.mu.Lock()
	on := !a.enabled[port]
	if on {
		a.enabled[port] = true
	} else {
		delete(a.enabled, port)
	}
	a.mu.Unlock()
	a.refresh()
}

func (a *guiApp) selectAll(on bool) {
	a.mu.Lock()
	a.checked = map[int64]bool{}
	if on {
		for p := range a.targets {
			a.checked[p] = true
		}
	}
	a.mu.Unlock()
	a.refresh()
}

func (a *guiApp) applyChecked(on bool) {
	a.mu.Lock()
	for p := range a.checked {
		if _, ok := a.targets[p]; !ok {
			continue
		}
		if on {
			a.enabled[p] = true
		} else {
			delete(a.enabled, p)
		}
	}
	a.mu.Unlock()
	a.refresh()
}

func (a *guiApp) startAll() {
	a.mu.Lock()
	for p := range a.targets {
		a.enabled[p] = true
	}
	a.mu.Unlock()
	a.refresh()
}

func (a *guiApp) startSelected() { a.applyChecked(true) }
func (a *guiApp) stopSelected()  { a.applyChecked(false) }

func (a *guiApp) stopAll() {
	a.mu.Lock()
	a.enabled = map[int64]bool{}
	a.mu.Unlock()
	a.refresh()
}

// copyPort puts just the port number on the clipboard, ready to paste after
// WindowsIP:. It also flashes a short confirmation in the status bar.
func (a *guiApp) copyPort(port int64) {
	s := strconv.FormatInt(port, 10)
	mygo.Clipboard.WriteText(s)
	a.mu.Lock()
	a.message = "已复制端口 " + s + " 到剪贴板"
	a.flashUntil = time.Now().Add(3 * time.Second)
	a.mu.Unlock()
	a.refresh()
}

// sortRows orders rows by the column ID the table header selected.
func sortRows(rows []rowView, order ui.SortOrder) {
	sort.SliceStable(rows, func(i, j int) bool {
		c := compareRows(rows[i], rows[j], order.Column)
		if order.Descending {
			return c > 0
		}
		return c < 0
	})
}

func compareRows(a, b rowView, col string) int {
	switch col {
	case "select":
		return cmp.Compare(boolKey(a.Checked), boolKey(b.Checked))
	case "port":
		return cmp.Compare(a.Port, b.Port)
	case "proc":
		return cmp.Compare(a.Proc, b.Proc)
	case "access":
		return cmp.Compare(accessKey(a), accessKey(b))
	case "status":
		return cmp.Compare(statusRank(a), statusRank(b))
	case "action":
		return cmp.Compare(boolKey(a.Enabled), boolKey(b.Enabled))
	default:
		return cmp.Compare(a.Port, b.Port)
	}
}

func boolKey(b bool) int {
	if b {
		return 1
	}
	return 0
}

func accessKey(r rowView) int64 {
	if r.ListenPort == 0 {
		return 1 << 30 // not forwarding: sort after real ports
	}
	return r.ListenPort
}

func statusRank(r rowView) int {
	switch {
	case r.Running:
		return 0
	case r.Enabled:
		return 1
	default:
		return 2
	}
}
