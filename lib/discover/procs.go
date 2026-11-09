package discover

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// ParseProcNetTCPInodes parses a /proc/net/tcp{,6}-style file and returns,
// for every LISTEN port, the socket inodes of its listening sockets. The
// inode is field 9; a port can have several (tcp + tcp6, SO_REUSEPORT).
func ParseProcNetTCPInodes(r io.Reader) map[int64][]uint64 {
	out := map[int64][]uint64{}
	sc := bufio.NewScanner(r)
	first := true
	for sc.Scan() {
		line := sc.Text()
		if first {
			first = false // header
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 10 || fields[3] != listenState {
			continue
		}
		local := fields[1]
		idx := strings.LastIndex(local, ":")
		if idx < 0 {
			continue
		}
		port, err := strconv.ParseUint(local[idx+1:], 16, 16)
		if err != nil || port == 0 || port > 65535 {
			continue
		}
		ino, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil || ino == 0 {
			continue
		}
		p := int64(port)
		if !containsUint(out[p], ino) {
			out[p] = append(out[p], ino)
		}
	}
	return out
}

func containsUint(xs []uint64, v uint64) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

// PortInodes merges the LISTEN inodes of /proc/net/tcp and /proc/net/tcp6.
func PortInodes() map[int64][]uint64 {
	out := map[int64][]uint64{}
	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		for port, inodes := range ParseProcNetTCPInodes(f) {
			for _, ino := range inodes {
				if !containsUint(out[port], ino) {
					out[port] = append(out[port], ino)
				}
			}
		}
		f.Close()
	}
	return out
}

// parseSocketLink recognizes a /proc/<pid>/fd symlink like "socket:[12345]".
func parseSocketLink(s string) (uint64, bool) {
	const prefix, suffix = "socket:[", "]"
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		return 0, false
	}
	ino, err := strconv.ParseUint(s[len(prefix):len(s)-len(suffix)], 10, 64)
	if err != nil || ino == 0 {
		return 0, false
	}
	return ino, true
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func collectInodes(pi map[int64][]uint64) map[uint64]bool {
	wanted := map[uint64]bool{}
	for _, inodes := range pi {
		for _, ino := range inodes {
			wanted[ino] = true
		}
	}
	return wanted
}

// fingerprint is a stable key for the set of listening sockets, independent
// of map iteration order, used to avoid rescanning /proc every tick.
func fingerprint(pi map[int64][]uint64) string {
	ports := make([]int64, 0, len(pi))
	for p := range pi {
		ports = append(ports, p)
	}
	sort.Slice(ports, func(i, j int) bool { return ports[i] < ports[j] })
	var b strings.Builder
	for _, p := range ports {
		inodes := append([]uint64(nil), pi[p]...)
		sort.Slice(inodes, func(i, j int) bool { return inodes[i] < inodes[j] })
		fmt.Fprintf(&b, "%d:", p)
		for _, ino := range inodes {
			fmt.Fprintf(&b, "%d,", ino)
		}
		b.WriteByte(';')
	}
	return b.String()
}

// socketInodeOwners finds, for each wanted socket inode, the name of a process
// holding that socket. It takes procRoot as a parameter so tests can use a
// temporary directory. The first process found wins when several share a fd.
func socketInodeOwners(wanted map[uint64]bool, procRoot string) map[uint64]string {
	owners := map[uint64]string{}
	if len(wanted) == 0 {
		return owners
	}
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return owners
	}
	for _, e := range entries {
		if !e.IsDir() || !isNumeric(e.Name()) {
			continue
		}
		pid := e.Name()
		fdDir := filepath.Join(procRoot, pid, "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue // EACCES, or the process exited
		}
		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}
			ino, ok := parseSocketLink(link)
			if !ok || !wanted[ino] {
				continue
			}
			if _, exists := owners[ino]; exists {
				continue
			}
			owners[ino] = processName(procRoot, pid)
		}
	}
	return owners
}

// processName prefers the real executable name. /proc/<pid>/comm is capped at
// 15 characters by the kernel, so a name like "sun-ray-nogui-linux-amd64"
// shows up there as "sun-ray-nogui-l". /proc/<pid>/exe and cmdline are not.
func processName(procRoot, pid string) string {
	dir := filepath.Join(procRoot, pid)
	if target, err := os.Readlink(filepath.Join(dir, "exe")); err == nil {
		if name := cleanProcName(filepath.Base(target)); name != "" {
			return name
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, "cmdline")); err == nil {
		if i := bytes.IndexByte(b, 0); i > 0 {
			if name := cleanProcName(filepath.Base(string(b[:i]))); name != "" {
				return name
			}
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, "comm")); err == nil {
		if name := strings.TrimSpace(string(b)); name != "" {
			return name
		}
	}
	return "unknown"
}

func cleanProcName(s string) string {
	// Linux appends " (deleted)" to a replaced executable.
	if i := strings.Index(s, " (deleted)"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	s = strings.TrimSpace(s)
	if s == "" || s == "." || s == "/" {
		return ""
	}
	return filepath.Base(s)
}

func copyProcs(src map[int64]string) map[int64]string {
	out := make(map[int64]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// ProcResolver caches the port -> process name map. The listening sockets of
// a machine rarely change, so a steady state does not rescan /proc every 3s.
type ProcResolver struct {
	mu     sync.Mutex
	root   string
	key    string
	cached map[int64]string
}

// NewProcResolver returns a resolver reading the live /proc filesystem.
func NewProcResolver() *ProcResolver {
	return &ProcResolver{root: "/proc"}
}

// Resolve returns the process name for each port that has a known owner.
// Ports whose owners cannot be found (a race, or a permission limit) are
// simply absent; the next tick recovers.
func (r *ProcResolver) Resolve(pi map[int64][]uint64) map[int64]string {
	key := fingerprint(pi)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cached != nil && r.key == key {
		return copyProcs(r.cached)
	}
	owners := socketInodeOwners(collectInodes(pi), r.root)
	result := map[int64]string{}
	for port, inodes := range pi {
		for _, ino := range inodes {
			if name, ok := owners[ino]; ok {
				result[port] = name
				break
			}
		}
	}
	r.key = key
	r.cached = copyProcs(result)
	return copyProcs(result)
}
