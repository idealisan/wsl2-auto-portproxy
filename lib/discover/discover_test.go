package discover

import (
	"reflect"
	"strings"
	"testing"
)

const sampleProcTCP = `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 12345 1 0000000000000000 100 0 0 10 0
   1: 00000000:0016 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 23456 1 0000000000000000 100 0 0 10 0
   2: 0100007F:0BB8 0100007F:1F90 01 00000000:00000000 00:00000000 00000000  1000        0 34567 1 0000000000000000 20 4 30 10 -1
   3: 0F02000A:0050 00000000:0000 06 00000000:00000000 00:00000000 00000000     0        0 45678 1 0000000000000000 100 0 0 10 0
`

func TestParseProcNetTCP(t *testing.T) {
	// 0x1F90=8080 (127.0.0.1 LISTEN), 0x16=22 (0.0.0.0 LISTEN),
	// 0x0BB8 is ESTABLISHED (skip), 0x50 is TIME_WAIT (skip).
	got := ParseProcNetTCP(strings.NewReader(sampleProcTCP))
	if !reflect.DeepEqual(got, []int64{8080, 22}) {
		t.Fatalf("ports = %v", got)
	}
}

const sampleProcTCP6 = `  sl  local_address                         rem_address                         st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000000000000000000001000000:0035 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 11111 1 0000000000000000 100 0 0 10 0
   1: 00000000000000000000000000000000:1F90 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 22222 1 0000000000000000 100 0 0 10 0
`

func TestParseProcNetTCP6(t *testing.T) {
	// 0x35=53, 0x1F90=8080, both LISTEN.
	got := ParseProcNetTCP(strings.NewReader(sampleProcTCP6))
	if !reflect.DeepEqual(got, []int64{53, 8080}) {
		t.Fatalf("ports = %v", got)
	}
}

func TestPortsFromProcFileMissing(t *testing.T) {
	if got := PortsFromProcFile("/nonexistent-proc-file"); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}
