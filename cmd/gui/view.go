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

		// Column headers, kept in the same widths as the rows below.
		ui.Row(c).Gap(8).Padding(6, 8).Children(func() {
			ui.Text(c, "选").Width(28)
			ui.Text(c, "端口").Width(70)
			ui.Text(c, "进程 / 服务").Grow(1)
			ui.Text(c, "状态").Width(90)
			ui.Text(c, "操作").Width(64)
		})

		ui.List(c, a.listState, len(snap.rows), func(i int) {
			r := snap.rows[i]
			ui.Row(c).Gap(8).Padding(6, 8).Children(func() {
				cb := ui.Checkbox(c.Key(r.Port), &r.Checked, "")
				cb.OnChange(func() { a.setChecked(r.Port, r.Checked) })

				ui.Textf(c, "%d", r.Port).Width(70)
				ui.Text(c, orDash(r.Proc)).Grow(1).SingleLine()
				ui.Text(c, rowStatus(r)).Width(90)

				label := "启动"
				if r.Enabled {
					label = "停止"
				}
				btn := ui.Button(c, label).Width(64)
				if r.Used {
					btn.Disabled(true)
				} else {
					btn.OnClick(func() { a.toggle(r.Port) })
				}
			})
		}).Grow(1).Children(func() {
			if len(snap.rows) == 0 {
				ui.Text(c, "未发现 WSL 监听端口，等待 agent 广播…").
					TextColor(c.Theme().TextMuted).Padding(12)
			}
		})

		// Bottom actions. Collection changes go through OnClick, which runs
		// after the view has finished building.
		ui.Row(c).Gap(8).Children(func() {
			ui.Button(c, "全选").OnClick(func() { a.selectAll(true) })
			ui.Button(c, "取消全选").OnClick(func() { a.selectAll(false) })
			ui.PrimaryButton(c, "启动所选").OnClick(a.startSelected)
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
	case r.Used:
		return "本机占用"
	case r.Running:
		return "已转发"
	case r.Enabled:
		return "启动中"
	default:
		return "未转发"
	}
}
