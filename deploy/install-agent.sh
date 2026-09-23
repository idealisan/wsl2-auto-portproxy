#!/usr/bin/env bash
# Install wslpp-agent as a systemd service inside WSL.
#
# Usage (inside WSL, as any user; sudo is requested automatically):
#   ./install-agent.sh [path-to-agent-binary]
#
# Default binary path: wslpp-agent-linux-amd64 next to this script.
# Example from Windows (PowerShell):
#   wsl --exec sudo bash /mnt/c/path/to/deploy/install-agent.sh /mnt/c/path/to/dist/wslpp-agent-linux-amd64
#
# What it does:
#   1. copies the binary to /usr/local/bin/wslpp-agent
#   2. installs wslpp-agent.service to /etc/systemd/system/
#   3. systemctl daemon-reload, enable and start it now
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_SRC="${1:-$SCRIPT_DIR/wslpp-agent-linux-amd64}"
BIN_DST="/usr/local/bin/wslpp-agent"
UNIT_SRC="$SCRIPT_DIR/wslpp-agent.service"
UNIT_DST="/etc/systemd/system/wslpp-agent.service"

if [ ! -f "$BIN_SRC" ]; then
  echo "error: agent binary not found: $BIN_SRC" >&2
  echo "usage: $0 [path-to-agent-binary]" >&2
  exit 1
fi
if [ ! -f "$UNIT_SRC" ]; then
  echo "error: unit file not found: $UNIT_SRC" >&2
  exit 1
fi

# Re-exec with sudo when not root.
if [ "$(id -u)" -ne 0 ]; then
  if ! command -v sudo >/dev/null 2>&1; then
    echo "error: need root (no sudo found), run as root instead" >&2
    exit 1
  fi
  exec sudo -- "$0" "$@"
fi

# systemd must be PID 1 (needs [boot] systemd=true in /etc/wsl.conf + wsl --shutdown).
if [ "$(ps -p 1 -o comm=)" != "systemd" ]; then
  echo "error: systemd is not running in this WSL distro." >&2
  echo "" >&2
  echo "Enable it first, then reboot WSL and re-run this script:" >&2
  echo "  1. append to /etc/wsl.conf:" >&2
  echo "       [boot]" >&2
  echo "       systemd=true" >&2
  echo "  2. from Windows PowerShell:  wsl --shutdown" >&2
  echo "  3. re-open WSL and re-run:   $0 $*" >&2
  exit 1
fi

echo "installing binary: $BIN_SRC -> $BIN_DST"
install -m 0755 "$BIN_SRC" "$BIN_DST"

echo "installing unit: $UNIT_SRC -> $UNIT_DST"
install -m 0644 "$UNIT_SRC" "$UNIT_DST"

systemctl daemon-reload
systemctl enable --now wslpp-agent.service

echo ""
systemctl --no-pager --full status wslpp-agent.service || true
echo ""
echo "logs:   journalctl -u wslpp-agent -f"
echo "remove: sudo systemctl disable --now wslpp-agent.service"
echo "        sudo rm -f $UNIT_DST $BIN_DST"
