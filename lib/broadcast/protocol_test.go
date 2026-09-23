package broadcast

import (
	"reflect"
	"testing"
)

func TestParseExample(t *testing.T) {
	ports, agent, err := Parse("ports\n22\n80\n443\nagent port\n3685\n")
	if err != nil {
		t.Fatalf("parse error: %s", err)
	}
	if !reflect.DeepEqual(ports, []int64{22, 80, 443}) {
		t.Fatalf("ports = %v", ports)
	}
	if agent != 3685 {
		t.Fatalf("agent = %d", agent)
	}
}

func TestBuildParseRoundTrip(t *testing.T) {
	in := []int64{443, 22, 22, 8080}
	got, agent, err := Parse(Build(in, 4567))
	if err != nil {
		t.Fatalf("parse error: %s", err)
	}
	if !reflect.DeepEqual(got, []int64{22, 443, 8080}) {
		t.Fatalf("ports = %v", got)
	}
	if agent != 4567 {
		t.Fatalf("agent = %d", agent)
	}
}

func TestParseMissingAgentPort(t *testing.T) {
	if _, _, err := Parse("ports\n22\n"); err == nil {
		t.Fatal("expected error for missing agent port")
	}
	if _, _, err := Parse("22\nagent port\n1\n"); err == nil {
		t.Fatal("expected error for missing ports header")
	}
}

func TestParseIgnoresUnknownLines(t *testing.T) {
	ports, agent, err := Parse("ports\nnotaport\n0\n70000\n22\nagent port\n3685\nextra stuff\n")
	if err != nil {
		t.Fatalf("parse error: %s", err)
	}
	if !reflect.DeepEqual(ports, []int64{22}) {
		t.Fatalf("ports = %v", ports)
	}
	if agent != 3685 {
		t.Fatalf("agent = %d", agent)
	}
}
