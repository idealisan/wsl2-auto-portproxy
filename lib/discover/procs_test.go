package discover

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseProcNetTCPInodes(t *testing.T) {
	// 0x1F90=8080 (127.0.0.1 LISTEN, inode 12345),
	// 0x16=22 (0.0.0.0 LISTEN, inode 23456); the ESTABLISHED and
	// TIME_WAIT rows must be skipped.
	got := ParseProcNetTCPInodes(strings.NewReader(sampleProcTCP))
	want := map[int64][]uint64{8080: {12345}, 22: {23456}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("inodes = %v, want %v", got, want)
	}
}

func TestParseProcNetTCPInodesZeroInode(t *testing.T) {
	const sample = "h\n" +
		"   0: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000 1000 0 0 1 000\n"
	got := ParseProcNetTCPInodes(strings.NewReader(sample))
	if len(got) != 0 {
		t.Fatalf("inode 0 must be skipped, got %v", got)
	}
}

func TestParseSocketLink(t *testing.T) {
	cases := []struct {
		in   string
		want uint64
		ok   bool
	}{
		{"socket:[12345]", 12345, true},
		{"socket:[0]", 0, false},
		{"socket:[abc]", 0, false},
		{"pipe:[123]", 0, false},
		{"socket:[12345", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := parseSocketLink(c.in)
		if got != c.want || ok != c.ok {
			t.Fatalf("parseSocketLink(%q) = %d,%v; want %d,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestIsNumeric(t *testing.T) {
	for _, s := range []string{"1", "100", "007"} {
		if !isNumeric(s) {
			t.Fatalf("%q should be numeric", s)
		}
	}
	for _, s := range []string{"", "self", "1a", "-1"} {
		if isNumeric(s) {
			t.Fatalf("%q should not be numeric", s)
		}
	}
}

func TestCollectInodes(t *testing.T) {
	got := collectInodes(map[int64][]uint64{22: {1, 2}, 80: {2, 3}})
	want := map[uint64]bool{1: true, 2: true, 3: true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("inodes = %v, want %v", got, want)
	}
}

func TestFingerprintStable(t *testing.T) {
	a := map[int64][]uint64{80: {1, 2}, 22: {3}}
	b := map[int64][]uint64{22: {3}, 80: {2, 1}}
	if fingerprint(a) != fingerprint(b) {
		t.Fatalf("fingerprint must not depend on map order: %q vs %q", fingerprint(a), fingerprint(b))
	}
	if fingerprint(a) == fingerprint(map[int64][]uint64{22: {3}}) {
		t.Fatal("fingerprint must change when the socket set changes")
	}
}

func writeProc(t *testing.T, root, pid, comm string, fds map[string]string) {
	t.Helper()
	dir := filepath.Join(root, pid)
	if err := os.MkdirAll(filepath.Join(dir, "fd"), 0755); err != nil {
		t.Fatal(err)
	}
	if comm != "" {
		if err := os.WriteFile(filepath.Join(dir, "comm"), []byte(comm+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for name, target := range fds {
		if err := os.Symlink(target, filepath.Join(dir, "fd", name)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSocketInodeOwners(t *testing.T) {
	root := t.TempDir()
	// pid 100 wins inode 12345 even though pid 200 shares the fd.
	writeProc(t, root, "100", "sshd", map[string]string{"3": "socket:[12345]"})
	writeProc(t, root, "200", "node", map[string]string{
		"4": "socket:[12345]",
		"5": "socket:[23456]",
	})
	// A process without a readable comm yields "unknown".
	writeProc(t, root, "300", "", map[string]string{"3": "socket:[99999]"})
	if err := os.MkdirAll(filepath.Join(root, "self"), 0755); err != nil {
		t.Fatal(err)
	}

	got := socketInodeOwners(map[uint64]bool{12345: true, 23456: true, 99999: true, 88888: true}, root)
	want := map[uint64]string{12345: "sshd", 23456: "node", 99999: "unknown"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("owners = %v, want %v", got, want)
	}
}

func TestProcResolverCache(t *testing.T) {
	root := t.TempDir()
	writeProc(t, root, "100", "sshd", map[string]string{"3": "socket:[12345]"})
	r := &ProcResolver{root: root}
	pi := map[int64][]uint64{22: {12345}}

	first := r.Resolve(pi)
	if !reflect.DeepEqual(first, map[int64]string{22: "sshd"}) {
		t.Fatalf("first resolve = %v", first)
	}

	// Same socket set -> cached, even though comm changed on disk.
	if err := os.WriteFile(filepath.Join(root, "100", "comm"), []byte("bash\n"), 0644); err != nil {
		t.Fatal(err)
	}
	second := r.Resolve(pi)
	if !reflect.DeepEqual(second, map[int64]string{22: "sshd"}) {
		t.Fatalf("cache miss, second resolve = %v", second)
	}

	// Socket set changes -> rescan and pick up the new name.
	third := r.Resolve(map[int64][]uint64{22: {12345}, 80: {23456}})
	if !reflect.DeepEqual(third, map[int64]string{22: "bash"}) {
		t.Fatalf("after fingerprint change, resolve = %v", third)
	}
}
