package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

func (a *guiApp) view(c *ui.Context) {
	// Take an immutable snapshot under the lock, then build the whole control
	// tree without touching shared state. Every callback re-locks on its own.
	a.mu.Lock()
	snap := a.snapshotLocked()
	a.mu.Unlock()

	a.listState.Key = func(i int) any { return snap.rows[i].ID }
	a.listState.Sort = &a.sort
	a.listState.Selection = &a.chosen

	// A Table gives us click-to-sort headers, draggable column edges and
	// Ctrl/Cmd-click, Shift-click and Ctrl+A multi-selection. IDs match the
	// switch in compareRows.
	cols := []ui.TableColumn{
		{Title: "选择", ID: "select", Width: 64, MinWidth: 56, MaxWidth: 96, Sortable: true},
		{Title: "WSL 端口", ID: "port", Width: 88, MinWidth: 70, MaxWidth: 160, Sortable: true},
		{Title: "WSL 环境", ID: "agent", Width: 150, MinWidth: 110, MaxWidth: 240, Sortable: true},
		{Title: "进程 / 服务", ID: "proc", MinWidth: 140, Sortable: true},
		{Title: "访问端口", ID: "access", Width: 96, MinWidth: 80, MaxWidth: 160, Sortable: true},
		{Title: "状态", ID: "status", Width: 84, MinWidth: 70, MaxWidth: 140, Sortable: true},
		{Title: "操作", ID: "action", Width: 84, MinWidth: 70, MaxWidth: 140, Sortable: true},
	}

	ui.Column(c).Fill().Padding(12).Gap(8).Children(func() {
		// Status bar.
		ui.Row(c).Gap(8).Children(func() {
			status := "agent 离线"
			if snap.online {
				status = fmt.Sprintf("agent 在线 (%d)", snap.agentCount)
			}
			ui.Text(c, status).Bold()
			ui.Text(c, "WSL: "+orDash(snap.wslIP)).Grow(1).TextColor(c.Theme().TextMuted)
			ui.Textf(c, "已转发 %d", snap.forwarded)
			if snap.version != "" {
				ui.Text(c, "v"+snap.version).TextColor(c.Theme().TextMuted)
			}
		})
		ui.Text(c, snap.message).TextColor(c.Theme().TextMuted).SingleLine()
		ui.Text(c, "点端口数字可复制到剪贴板").TextColor(c.Theme().TextMuted).SingleLine()

		ui.Table(c, a.listState, cols, len(snap.rows), func(row, col int) {
			r := snap.rows[row]
			switch col {
			case 0:
				checked := a.chosen.Has(r.ID)
				cb := ui.Checkbox(c.Key(r.ID), &checked, "")
				cb.OnChange(func() { a.setChecked(r.ID, checked) })
			case 1:
				ui.Textf(c, "%d", r.Port)
			case 2:
				ui.Text(c, r.WslLabel).SingleLine()
			case 3:
				// No SingleLine(): long process names wrap and the row grows
				// instead of being clipped.
				ui.Text(c, orDash(r.Proc))
			case 4:
				if !r.Running {
					ui.Text(c, "-").TextColor(c.Theme().TextMuted)
					break
				}
				// Just the number, no colon. Click to copy it.
				ui.ButtonBase(c).Tooltip("点击复制端口").OnClick(func() {
					a.copyPort(r.ListenPort)
				}).Children(func() {
					t := ui.Textf(c, "%d", r.ListenPort)
					if r.ListenPort != r.Port {
						t.Bold()
					}
				})
			case 5:
				ui.Text(c, rowStatus(r))
			case 6:
				label := "启动"
				if r.Enabled {
					label = "停止"
				}
				ui.Button(c, label).OnClick(func() { a.toggle(r.ID) })
			}
		}).Grow(1)

		// Bottom actions. Collection changes go through OnClick, which runs
		// after the view has finished building.
		ui.Row(c).Gap(6).Children(func() {
			ui.Button(c, "全选").OnClick(func() { a.selectAll(true) })
			ui.Button(c, "全不选").OnClick(func() { a.selectAll(false) })
			ui.PrimaryButton(c, "全部启动").OnClick(a.startAll)
			ui.Button(c, "启动所选").OnClick(a.startSelected)
			ui.Button(c, "停止所选").OnClick(a.stopSelected)
			ui.Button(c, "全部停止").OnClick(a.stopAll)
			ui.Button(c, "刷新").OnClick(a.refresh)
		})
	})
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func rowStatus(r rowView) string {
	switch {
	case r.Running:
		return "已转发"
	case r.Enabled:
		return "启动中"
	default:
		return "未转发"
	}
}
