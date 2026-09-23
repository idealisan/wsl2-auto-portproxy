// Package service reports locally used TCP ports so forwarding can skip
// conflicts. The mechanism is per-OS (see localports_*.go); the parsers
// below are pure functions shared by all platforms.
package service

import (
	"bufio"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// windowsNetstatRE matches only wildcard binds (0.0.0.0 / [::]),
// mirroring the rule that a 127.0.0.1-only listener does not claim the port.
var windowsNetstatRE = regexp.MustCompile(`TCP\s+(\[::\]:|0\.0\.0\.0:)(\d{2,5})`)

// parseWindowsNetstat extracts wildcard-bound TCP ports from `netstat -ano` output.
func parseWindowsNetstat(text string) []int64 {
	seen := map[int64]bool{}
	var out []int64
	for _, m := range windowsNetstatRE.FindAllStringSubmatch(text, -1) {
		p, _ := strconv.ParseInt(m[2], 10, 64)
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// parseDarwinNetstat extracts wildcard-bound TCP ports from
// `netstat -an -p tcp` output (local address looks like `*.8080`).
// Only `*` binds claim the port; 127.0.0.1-only listeners do not.
func parseDarwinNetstat(r io.Reader) []int64 {
	seen := map[int64]bool{}
	var out []int64
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 6 || !strings.HasPrefix(f[0], "tcp") {
			continue
		}
		if f[len(f)-1] != "LISTEN" {
			continue
		}
		local := f[3]
		i := strings.LastIndex(local, ".")
		if i < 0 || local[:i] != "*" {
			continue
		}
		n, err := strconv.Atoi(local[i+1:])
		if err != nil || n <= 0 || n > 65535 {
			continue
		}
		if !seen[int64(n)] {
			seen[int64(n)] = true
			out = append(out, int64(n))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
