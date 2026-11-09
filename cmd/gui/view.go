package main

import (
	"github.com/egoist/mygo/ui"
)

func (a *guiApp) view(c *ui.Context) {
	// Take an immutable snapshot under the lock, then build the whole control
	// tree without touching shared state. Every callback re-locks on its own.
	a.mu.Lock()
	snap := a.snapshotLocked()
	a.mu.Unlock()

	a.listState.Key = func(i int) any { return snap.rows[i].Port }
	a.listState.Sort = &a.sort

	// A Table gives us click-to-sort headers and draggable column edges.
	// Every column is sortable and resizable; Width 0 shares the leftover
	// room. IDs match the switch in compareRows.
	cols := []ui.TableColumn{
		{Title: "选择", ID: "select", Width: 64, MinWidth: 56, MaxWidth: 96, Sortable: true},
		{Title: "WSL 端口", ID: "port", Width: 90, MinWidth: 70, MaxWidth: 160, Sortable: true},
		{Title: "进程 / 服务", ID: "proc", MinWidth: 150, Sortable: true},
		{Title: "访问端口", ID: "access", Width: 100, MinWidth: 80, MaxWidth: 160, Sortable: true},
		{Title: "状态", ID: "status", Width: 90, MinWidth: 70, MaxWidth: 140, Sortable: true},
		{Title: "操作", ID: "action", Width: 90, MinWidth: 70, MaxWidth: 140, Sortable: true},
	}

	ui.Column(c).Fill().Padding(12).Gap(8).Children(func() {
		// Status bar.
		ui.Row(c).Gap(8).Children(func() {
			status := "agent 离线"
			if snap.online {
				status = "agent 在线"
			}
			ui.Text(c, status).Bold()
			ui.Text(c, "WSL: "+orDash(snap.wslIP)).Grow(1).TextColor(c.Theme().TextMuted)
			ui.Textf(c, "已转发 %d", snap.forwarded)
			if snap.version != "" {
				ui.Text(c, "v"+snap.version).TextColor(c.Theme().TextMuted)
			}
		})
		ui.Text(c, snap.message).TextColor(c.Theme().TextMuted).SingleLine()
		ui.Text(c, "本机端口被占用时自动改用随机端口；点「访问端口」数字复制，拖列头边缘调宽度，点列头排序。").
			TextColor(c.Theme().TextMuted).SingleLine()

		ui.Table(c, a.listState, cols, len(snap.rows), func(row, col int) {
			r := snap.rows[row]
			switch col {
			case 0:
				cb := ui.Checkbox(c.Key(r.Port), &r.Checked, "")
				cb.OnChange(func() { a.setChecked(r.Port, r.Checked) })
			case 1:
				ui.Textf(c, "%d", r.Port)
			case 2:
				// No SingleLine(): long process names wrap and the row grows
				// instead of being clipped.
				ui.Text(c, orDash(r.Proc))
			case 3:
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
						t.Bold() // fallback port, because the host one was busy
					}
				})
			case 4:
				ui.Text(c, rowStatus(r))
			case 5:
				label := "启动"
				if r.Enabled {
					label = "停止"
				}
				ui.Button(c, label).OnClick(func() { a.toggle(r.Port) })
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
