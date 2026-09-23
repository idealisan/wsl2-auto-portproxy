package service

import (
	"os/exec"
	"regexp"
	"strconv"
)

// GetWindowsHostPorts returns TCP ports already listening on all Windows
// interfaces. Forwarding skips these to avoid conflicts.
func GetWindowsHostPorts() ([]int64, error) {
	cmd := exec.Command("cmd", "/c", "Netstat", "-ano", "|", "findstr", "LISTENING")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	reg := regexp.MustCompile(`TCP\s+(\[::\]:|0.0.0.0:)(\d{2,5})`)
	rets := reg.FindAllStringSubmatch(string(output), -1)
	seen := map[int64]bool{}
	var windowsPorts []int64
	for _, ret := range rets {
		p, _ := strconv.ParseInt(ret[2], 10, 0)
		if !seen[p] {
			seen[p] = true
			windowsPorts = append(windowsPorts, p)
		}
	}
	return windowsPorts, nil
}
