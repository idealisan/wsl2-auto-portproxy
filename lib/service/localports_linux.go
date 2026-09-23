//go:build linux
// +build linux

package service

import (
	"github.com/HobaiRiku/wsl2-auto-portproxy/lib/discover"
)

// GetLocalUsedPorts returns locally listening TCP ports via /proc,
// no external command needed.
func GetLocalUsedPorts() ([]int64, error) {
	return discover.LocalTCPPorts(nil), nil
}
