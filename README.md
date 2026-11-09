# wsl2-auto-portProxy

`wslpp` forwards every TCP port listening inside WSL2 — including `127.0.0.1/::1`
— to all interfaces of the Windows host, so devices on your LAN can reach your
WSL services. Zero config: run one agent in WSL, one proxy on Windows.

> **Security warning**: loopback-only services were never meant to leave the
> machine. Once forwarded, they are reachable from your LAN. Only use this on
> networks you trust.

## How it works

WSL2 lives behind a Hyper-V NAT with a shifting IP, and the built-in
`wslhost.exe` forwarding only listens on Windows loopback. `wslpp` closes both
gaps with two pieces:

- `wslpp-agent` (Linux, runs in WSL): scans all local TCP listeners every 3s
  via `/proc/net/tcp*` (any bind address), resolves each listener's owning
  process from `/proc/<pid>/fd` + `/proc/<pid>/exe` (with cmdline/comm
  fallbacks), then broadcasts
  `ports + agent-port + process names` on the fixed shared UDP port **1033**
  (global broadcast + subnet broadcast + host unicast for NAT reliability),
  and serves a single random TCP channel port. Each inbound connection sends
  one `"PORT\n"` line and is bridged to `127.0.0.1:PORT` (falling back to
  `::1`), so loopback-only services work without `--host 0.0.0.0`.
- `wslpp` (Windows, runs on the host): listens on UDP `:1033`, learns
  `{wslIp, ports, agentPort, process names}` from each broadcast's source
  address (no `wsl` CLI calls, immune to IP drift), listens on `:P` per port,
  and dials the agent channel per client connection.

There are no config files. Sources silent for over 9s are dropped with
their proxies. Port availability is checked by binding, with no external
command. Protocol details live in `docs/001-linux-agent-plan.md`.

## Install

### 1. Linux agent (systemd)

Download `wslpp-agent-<os>-<arch>` from
[Releases](https://github.com/HobaiRiku/wsl2-auto-portproxy/releases),
or build it with `make build-agent`. Then inside WSL install and enable it
(requires systemd, i.e. `[boot] systemd=true` in `/etc/wsl.conf`):

```bash
sudo bash ./deploy/install-agent.sh ./dist/wslpp-agent-linux-amd64
```

Logs: `journalctl -u wslpp-agent -f`.
Remove: `sudo systemctl disable --now wslpp-agent.service`
plus `sudo rm -f /etc/systemd/system/wslpp-agent.service /usr/local/bin/wslpp-agent`.

### 2. Windows proxy

Download `wslpp-windows-<arch>.exe` from
[Releases](https://github.com/HobaiRiku/wsl2-auto-portproxy/releases)
and run it (add it to startup for always-on forwarding). Logs go to stdout.

### 3. GUI (Windows / macOS)

Download `wslpp-gui-windows-<arch>.exe` for Windows, or
`wslpp-gui-darwin-<arch>.app.zip` for macOS (unzip it and drag
`wslpp-gui.app` into Applications). The macOS bundle is unsigned, so on
first launch use right-click → Open, or run
`xattr -d com.apple.quarantine wslpp-gui.app`. Both builds share the same
native mygo UI. The icon is drawn by `tools/genicon` (two arrows for ports
forwarded in both directions) and embedded in both the Windows exe and the
macOS bundle.

The GUI lists every WSL listening port with its process name and forwards
**nothing** until you choose:

- tick rows and press **启动所选** / **停止所选**, use each row's
  **启动/停止** button, or use **全部启动** / **全部停止** for every port.
  **全选** / **全不选** control the tick boxes; Ctrl/Cmd-click and Shift-click
  select multiple rows directly.
- the **访问端口** column shows the host port as a plain number. Click it to
  copy to the clipboard, then paste after `WindowsIP:`.
- process names come from the new agent in step 1 and are read from
  `/proc/<pid>/exe`, so long names are not truncated to 15 characters; an
  older agent leaves the column empty.

Install an agent in every WSL distro you want to reach. All of them appear in
the same GUI; the **WSL 环境** column shows which one a row came from, even
when several distros share one IP or listen on the same port.

The GUI and the headless `wslpp.exe` are separate programs — run one or the
other, not both (they would compete for the same host ports).

> The same security warning applies: forwarding a loopback-only service makes
> it reachable from your LAN.

## Releases

A GitHub Actions workflow builds and publishes on every `v*` tag:
`wslpp` and `wslpp-agent` for windows / linux / darwin × amd64 / arm64,
`wslpp-gui-windows-*.exe` for Windows, `wslpp-gui-darwin-*.app.zip` for
macOS, and `wslpp-gui.ico`
(17 files, see `.github/workflows/release.yml`). Every binary is built with
`-trimpath -buildvcs=false -s -w` to strip symbols, DWARF and local paths;
no packing, compression or obfuscation is used, which antivirus products
often flag.

## Develop

Development requires Go 1.27.3+ (the GUI depends on mygo v0.8.3).

```bash
make build        # wslpp.exe (windows/amd64) -> dist/
make build-agent  # agent (linux amd64+arm64) -> dist/
make build-gui    # wslpp-gui.exe (windows/amd64) -> dist/
make build-all    # all three
go run ./tools/genicon -out build/icons  # icon.png / .icns / .ico
go test ./...     # unit tests (protocol, /proc inodes, process names, proxy fallback)
```

## Notes

- Microsoft forwards WSL ports via `wslhost.exe` when
  `localhostForwarding=true` (default), but only on host loopback — that is
  exactly the gap `wslpp` fills. See
  [wsl-config](https://learn.microsoft.com/en-us/windows/wsl/wsl-config).
- [WSLHostPatcher](https://github.com/CzBiX/WSLHostPatcher) patches
  `wslhost.exe` instead — a more efficient alternative if you only need
  Windows-local access.
- UDP forwarding is not implemented yet.

## License

MIT
