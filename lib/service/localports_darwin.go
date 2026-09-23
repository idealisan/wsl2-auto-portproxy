//go:build darwin
// +build darwin

package service

import (
	"bytes"
	"os/exec"
)

// GetLocalUsedPorts returns wildcard-bound TCP ports via netstat.
func GetLocalUsedPorts() ([]int64, error) {
	cmd := exec.Command("netstat", "-an", "-p", "tcp")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseDarwinNetstat(bytes.NewReader(output)), nil
}
