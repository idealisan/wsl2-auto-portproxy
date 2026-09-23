// Package discover enumerates local TCP listening ports inside WSL.
// All bound addresses (127.0.0.1, ::1, 0.0.0.0, ::, specific IPs) are
// included: the agent bridges loopback-only ports, so no filtering here.
package discover

import (
	"bufio"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

// listenState is the TCP state code for LISTEN in /proc/net/tcp*.
const listenState = "0A"

// LocalTCPPorts returns all locally listening TCP ports, excluding any in exclude.
func LocalTCPPorts(exclude map[int]bool) []int64 {
	seen := map[int64]bool{}
	var out []int64
	add := func(ports []int64) {
		for _, p := range ports {
			if exclude[int(p)] || seen[p] {
				continue
			}
			seen[p] = true
			out = append(out, p)
		}
	}
	add(PortsFromProcFile("/proc/net/tcp"))
	add(PortsFromProcFile("/proc/net/tcp6"))
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// PortsFromProcFile parses a /proc/net/tcp{,6}-style file. Missing files yield nil.
func PortsFromProcFile(path string) []int64 {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	return ParseProcNetTCP(f)
}

// ParseProcNetTCP parses /proc/net/tcp{,6} content and returns LISTEN ports.
func ParseProcNetTCP(r io.Reader) []int64 {
	var out []int64
	seen := map[int64]bool{}
	sc := bufio.NewScanner(r)
	first := true
	for sc.Scan() {
		line := sc.Text()
		if first {
			first = false // header: sl local_address rem_address st ...
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[3] != listenState {
			continue
		}
		local := fields[1]
		idx := strings.LastIndex(local, ":")
		if idx < 0 {
			continue
		}
		n, err := strconv.ParseUint(local[idx+1:], 16, 16)
		if err != nil || n == 0 || n > 65535 {
			continue
		}
		if !seen[int64(n)] {
			seen[int64(n)] = true
			out = append(out, int64(n))
		}
	}
	return out
}
