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
	ID         string // (agent, port) identity
	Port       int64  // port listening inside WSL
	ListenPort int64  // port actually bound on Windows (0 when not forwarding)
	Proc       string
	WslLabel   string // which WSL environment this row came from
	Checked    bool
	Enabled    bool
	Running    bool
}

type snapshot struct {
	rows       []rowView
	online     bool
	wslIP      string
	agentCount int
	forwarded  int
	message    string
	version    string
}

type guiApp struct {
	mu        sync.Mutex
	win       *mygo.Window
	listState *ui.ListState

	targets    map[string]wdiscovery.Target // by ID
	chosen     ui.Selection[string]         // multi-selection (Ctrl/Cmd click)
	enabled    map[string]bool              // ports the user asked to forward
	running    map[string]int64             // ID -> actual Windows listen port
	agentCount int
	online     bool
	wslIP      string
	message    string

	// sort and chosen are written by the table on the UI thread and read by
	// snapshotLocked, which also runs on the UI thread, so they need no lock.
	sort       ui.SortOrder
	flashUntil time.Time
}

func newApp() *guiApp {
	return &guiApp{
		listState: &ui.ListState{},
		targets:   map[string]wdiscovery.Target{},
		enabled:   map[string]bool{},
		running:   map[string]int64{},
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
	ids := make([]string, 0, len(a.targets))
	for id := range a.targets {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	rows := make([]rowView, 0, len(ids))
	forwarded := 0
	for _, id := range ids {
		t := a.targets[id]
		r := rowView{
			ID:         id,
			Port:       t.Port,
			ListenPort: a.running[id],
			Proc:       t.Proc,
			WslLabel:   fmt.Sprintf("%s:%d", t.WslIP, t.AgentPort),
			Checked:    a.chosen.Has(id),
			Enabled:    a.enabled[id],
			Running:    a.running[id] != 0,
		}
		if r.Running {
			forwarded++
		}
		rows = append(rows, r)
	}
	sortRows(rows, a.sort)
	return snapshot{
		rows:       rows,
		online:     a.online,
		wslIP:      a.wslIP,
		agentCount: a.agentCount,
		forwarded:  forwarded,
		message:    a.message,
		version:    version,
	}
}

// applyState publishes the result of a reconcile pass and refreshes the view.
// It never touches chosen: that is owned by the UI thread's table selection.
func (a *guiApp) applyState(desired []wdiscovery.Target, running map[string]int64,
	online bool, wslIP string, agentCount int, errs []string) {
	a.mu.Lock()
	a.targets = make(map[string]wdiscovery.Target, len(desired))
	for _, t := range desired {
		a.targets[t.ID] = t
	}
	a.running = running
	a.agentCount = agentCount
	a.online = online
	a.wslIP = wslIP
	switch {
	case len(errs) > 0:
		a.message = strings.Join(errs, "；")
	case time.Now().Before(a.flashUntil):
		// Keep the short-lived message set by copyPort for a few seconds.
	case !online:
		a.message = "等待 WSL agent 广播…（请确认 wslpp-agent 正在 WSL 内运行）"
	default:
		a.message = fmt.Sprintf("发现 %d 个监听端口（%d 个 WSL 环境），已转发 %d 个",
			len(desired), agentCount, len(running))
	}
	a.mu.Unlock()
	a.refresh()
}

// setChecked syncs a checkbox click into the table's selection. Ctrl/Cmd
// click, Shift click and Ctrl+A are handled by the table directly.
func (a *guiApp) setChecked(id string, on bool) {
	if on {
		a.chosen.Add(id)
	} else {
		a.chosen.Remove(id)
	}
	a.refresh()
}

func (a *guiApp) toggle(id string) {
	a.mu.Lock()
	on := !a.enabled[id]
	if on {
		a.enabled[id] = true
	} else {
		delete(a.enabled, id)
	}
	a.mu.Unlock()
	a.refresh()
}

func (a *guiApp) selectAll(on bool) {
	a.chosen.Clear()
	if on {
		a.mu.Lock()
		for id := range a.targets {
			a.chosen.Add(id)
		}
		a.mu.Unlock()
	}
	a.refresh()
}

func (a *guiApp) applyChecked(on bool) {
	var ids []string
	for id := range a.chosen.All() {
		ids = append(ids, id)
	}
	a.mu.Lock()
	for _, id := range ids {
		if _, ok := a.targets[id]; !ok {
			continue
		}
		if on {
			a.enabled[id] = true
		} else {
			delete(a.enabled, id)
		}
	}
	a.mu.Unlock()
	a.refresh()
}

func (a *guiApp) startAll() {
	a.mu.Lock()
	for id := range a.targets {
		a.enabled[id] = true
	}
	a.mu.Unlock()
	a.refresh()
}

func (a *guiApp) startSelected() { a.applyChecked(true) }
func (a *guiApp) stopSelected()  { a.applyChecked(false) }

func (a *guiApp) stopAll() {
	a.mu.Lock()
	a.enabled = map[string]bool{}
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
	case "agent":
		return cmp.Compare(a.WslLabel, b.WslLabel)
	case "proc":
		return cmp.Compare(a.Proc, b.Proc)
	case "access":
		return cmp.Compare(accessKey(a), accessKey(b))
	case "status":
		return cmp.Compare(statusRank(a), statusRank(b))
	case "action":
		return cmp.Compare(boolKey(a.Enabled), boolKey(b.Enabled))
	default:
		return cmp.Compare(a.ID, b.ID)
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
