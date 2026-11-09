package wdiscovery

import (
	"testing"
	"time"
)

func withSources(t *testing.T, s map[string]*source) {
	t.Helper()
	mu.Lock()
	old := sources
	sources = s
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		sources = old
		mu.Unlock()
	})
}

// Two WSL distros can share one IP. Their agent channel ports tell them
// apart, and the same WSL port must show up once per agent.
func TestSnapshotKeepsEveryAgent(t *testing.T) {
	withSources(t, map[string]*source{
		"10.0.0.2/1111": {
			ip: "10.0.0.2", ports: []int64{22, 80}, agentPort: 1111,
			procs: map[int64]string{22: "sshd", 80: "nginx"}, lastSeen: time.Now(),
		},
		"10.0.0.2/2222": {
			ip: "10.0.0.2", ports: []int64{22, 3000}, agentPort: 2222,
			procs: map[int64]string{22: "sshd"}, lastSeen: time.Now(),
		},
	})

	got := Snapshot()
	if len(got) != 4 {
		t.Fatalf("want 4 targets (2 agents x 2 ports), got %d: %+v", len(got), got)
	}
	ids := map[string]bool{}
	n22, nProc := 0, 0
	for _, x := range got {
		if ids[x.ID] {
			t.Fatalf("duplicate ID %q", x.ID)
		}
		ids[x.ID] = true
		if x.Port == 22 {
			n22++
		}
		if x.Proc != "" {
			nProc++
		}
	}
	if n22 != 2 {
		t.Fatalf("port 22 should appear once per agent, got %d", n22)
	}
	if nProc == 0 {
		t.Fatal("process names should be carried through")
	}
}

func TestSnapshotDropsExpiredSources(t *testing.T) {
	withSources(t, map[string]*source{
		"10.0.0.3/1": {
			ip: "10.0.0.3", ports: []int64{8080}, agentPort: 1,
			lastSeen: time.Now().Add(-10 * time.Second),
		},
	})
	if got := Snapshot(); len(got) != 0 {
		t.Fatalf("expired source should be dropped, got %+v", got)
	}
}
