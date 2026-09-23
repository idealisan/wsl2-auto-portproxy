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
  via `/proc/net/tcp*` (any bind address), broadcasts
  `ports + agent-port` on the fixed shared UDP port **1033**
  (global broadcast + subnet broadcast + host unicast for NAT reliability),
  and serves a single random TCP channel port. Each inbound connection sends
  one `"PORT\n"` line and is bridged to `127.0.0.1:PORT` (falling back to
  `::1`), so loopback-only services work without `--host 0.0.0.0`.
- `wslpp` (Windows, runs on the host): listens on UDP `:1033`, learns
  `{wslIp, ports, agentPort}` from each broadcast's source address (no `wsl`
  CLI calls, immune to IP drift), listens on `:P` per port, and dials the
  agent channel per client connection.

There are no config files. Ports already used on the local machine are
skipped; sources silent for over 9s are dropped with their proxies.
Protocol details live in `docs/001-linux-agent-plan.md`.

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

## Releases

A GitHub Actions workflow builds and publishes on every `v*` tag:
`wslpp` and `wslpp-agent` for windows / linux / darwin × amd64 / arm64
(12 files, see `.github/workflows/release.yml`).

## Develop

```bash
make build        # wslpp.exe (windows/amd64) -> dist/
make build-agent  # agent (linux amd64+arm64) -> dist/
make build-all    # both
go test ./...     # unit tests (protocol, /proc and netstat parsers)
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
