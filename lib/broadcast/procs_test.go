package broadcast

import (
	"reflect"
	"testing"
)

func TestBuildWithProcs(t *testing.T) {
	got := BuildWithProcs([]int64{3000, 22, 80}, 3685, map[int64]string{
		22:   "sshd",
		80:   "nginx",
		3000: "node server",
	})
	want := "ports\n22\n80\n3000\nagent port\n3685\n" +
		"procs\nproc 22 sshd\nproc 80 nginx\nproc 3000 node server\n"
	if got != want {
		t.Fatalf("payload =\n%q\nwant\n%q", got, want)
	}
}

// A payload with procs must parse exactly like one without: that is the
// backwards-compatibility contract with older Windows builds.
func TestBuildWithProcsCompatibleWithOldParse(t *testing.T) {
	in := []int64{3000, 22, 80}
	procs := map[int64]string{22: "sshd", 80: "nginx", 3000: "node server"}

	portsWith, agentWith, err := Parse(BuildWithProcs(in, 3685, procs))
	if err != nil {
		t.Fatalf("parse with procs: %s", err)
	}
	portsWithout, agentWithout, err := Parse(Build(in, 3685))
	if err != nil {
		t.Fatalf("parse without procs: %s", err)
	}
	if !reflect.DeepEqual(portsWith, portsWithout) || agentWith != agentWithout {
		t.Fatalf("old Parse changed: with=%v/%d without=%v/%d",
			portsWith, agentWith, portsWithout, agentWithout)
	}
	if !reflect.DeepEqual(portsWith, []int64{22, 80, 3000}) || agentWith != 3685 {
		t.Fatalf("unexpected parse result: %v/%d", portsWith, agentWith)
	}
}

func TestParseProcs(t *testing.T) {
	got := ParseProcs("ports\n22\n80\nagent port\n3685\n" +
		"procs\nproc 22 sshd\nproc 80 node server\n")
	want := map[int64]string{22: "sshd", 80: "node server"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("procs = %v, want %v", got, want)
	}
}

func TestParseProcsOldAgent(t *testing.T) {
	if got := ParseProcs(Build([]int64{22, 80}, 3685)); len(got) != 0 {
		t.Fatalf("old payload should have no procs, got %v", got)
	}
}

func TestParseProcsIgnoresMalformed(t *testing.T) {
	got := ParseProcs("procs\n" +
		"proc 22 sshd\n" +
		"proc x bash\n" +
		"proc 70000 big\n" +
		"proc 80\n" +
		"proc 80 \n" +
		"proc 443 python3\n")
	want := map[int64]string{22: "sshd", 443: "python3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("procs = %v, want %v", got, want)
	}
}

func TestBuildWithProcsSkipsEmptyAndSelf(t *testing.T) {
	got := BuildWithProcs([]int64{22, 443, 3685}, 3685, map[int64]string{
		22:   "sshd",
		443:  "",
		3685: "wslpp-agent",
	})
	want := "ports\n22\n443\n3685\nagent port\n3685\nprocs\nproc 22 sshd\n"
	if got != want {
		t.Fatalf("payload =\n%q\nwant\n%q", got, want)
	}
}
