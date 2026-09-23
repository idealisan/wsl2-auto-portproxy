//go:build windows
// +build windows

package service

import (
	"os/exec"
)

// GetLocalUsedPorts returns TCP ports already listening on all Windows
// interfaces. Forwarding skips these to avoid conflicts.
func GetLocalUsedPorts() ([]int64, error) {
	cmd := exec.Command("cmd", "/c", "Netstat", "-ano", "|", "findstr", "LISTENING")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseWindowsNetstat(string(output)), nil
}
