package service

import (
	"reflect"
	"strings"
	"testing"
)

const sampleWindowsNetstat = `Active Connections

  Proto  Local Address          Foreign Address        State
  TCP    0.0.0.0:135            0.0.0.0:0              LISTENING
  TCP    0.0.0.0:445            0.0.0.0:0              LISTENING
  TCP    127.0.0.1:49632        0.0.0.0:0              LISTENING
  TCP    [::]:135               [::]:0                 LISTENING
  TCP    192.168.1.10:139       0.0.0.0:0              LISTENING
`

func TestParseWindowsNetstat(t *testing.T) {
	// Only wildcard binds (0.0.0.0 / [::]) claim the port:
	// 127.0.0.1:49632 and the specific-IP 139 bind are skipped.
	got := parseWindowsNetstat(sampleWindowsNetstat)
	if !reflect.DeepEqual(got, []int64{135, 445}) {
		t.Fatalf("ports = %v", got)
	}
}

const sampleDarwinNetstat = `Active Internet connections (including servers)
Proto Recv-Q Send-Q  Local Address          Foreign Address        (state)
tcp4       0      0  *.8080                 *.*                    LISTEN
tcp4       0      0  127.0.0.1.5432         *.*                    LISTEN
tcp6       0      0  *.22                   *.*                    LISTEN
tcp4       0      0  192.168.1.5.3000       192.168.1.6.52086      ESTABLISHED
`

func TestParseDarwinNetstat(t *testing.T) {
	// Only `*` binds claim the port: 127.0.0.1.5432 skipped, ESTABLISHED skipped.
	got := parseDarwinNetstat(strings.NewReader(sampleDarwinNetstat))
	if !reflect.DeepEqual(got, []int64{22, 8080}) {
		t.Fatalf("ports = %v", got)
	}
}
