// Package broadcast defines the fixed UDP discovery protocol between
// wslpp-agent (Linux) and wslpp.exe (Windows).
//
// The agent broadcasts on the fixed, shared (non-exclusive) UDP port 1033.
// Payload is plain text, LF separated:
//
//	ports
//	22
//	80
//	443
//	agent port
//	3685
//
// "agent port" is the agent's random TCP channel port. Windows dials
// WSL_IP:agentPort for every forwarded connection.
package broadcast

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Port is the fixed shared UDP discovery port.
const Port = 1033

// Build renders the broadcast payload for the given TCP ports and agent channel port.
// Ports are sorted for determinism.
func Build(ports []int64, agentPort int) string {
	sorted := append([]int64(nil), ports...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	var b strings.Builder
	b.WriteString("ports\n")
	for _, p := range sorted {
		fmt.Fprintf(&b, "%d\n", p)
	}
	b.WriteString("agent port\n")
	fmt.Fprintf(&b, "%d\n", agentPort)
	return b.String()
}

// Parse parses a broadcast payload. Unknown lines are ignored for forward compatibility.
func Parse(payload string) (ports []int64, agentPort int, err error) {
	seenPorts := map[int64]bool{}
	expectAgent := false
	sawHeader := false
	for _, line := range strings.Split(payload, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if expectAgent {
			n, convErr := strconv.Atoi(line)
			if convErr != nil || n <= 0 || n > 65535 {
				return nil, 0, fmt.Errorf("invalid agent port %q", line)
			}
			agentPort = n
			expectAgent = false
			continue
		}
		switch {
		case line == "ports":
			sawHeader = true
		case line == "agent port":
			expectAgent = true
		default:
			n, convErr := strconv.Atoi(line)
			if convErr != nil || n <= 0 || n > 65535 {
				continue // ignore unknown lines
			}
			if !seenPorts[int64(n)] {
				seenPorts[int64(n)] = true
				ports = append(ports, int64(n))
			}
		}
	}
	if !sawHeader {
		return nil, 0, fmt.Errorf("missing ports header")
	}
	if expectAgent {
		return nil, 0, fmt.Errorf("missing agent port value")
	}
	if agentPort <= 0 {
		return nil, 0, fmt.Errorf("missing agent port")
	}
	sort.Slice(ports, func(i, j int) bool { return ports[i] < ports[j] })
	return ports, agentPort, nil
}
