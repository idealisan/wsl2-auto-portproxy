//go:build windows
// +build windows

package proxy

import (
	"encoding/binary"
	"errors"
	"net"
	"syscall"
	"unsafe"
)

var (
	iphlpapi            = syscall.NewLazyDLL("iphlpapi.dll")
	getExtendedTcpTable = iphlpapi.NewProc("GetExtendedTcpTable")
)

const (
	tcpTableOwnerPIDListener = 3
	afInet                   = 2
	errorInsufficientBuffer  = 122
)

var errPortBusy = errors.New("port already in use")

// listenTCP checks the system's listening table before binding. Windows lets
// a wildcard bind (0.0.0.0:P) coexist with a specific-IP one (127.0.0.1:P),
// so a plain bind would report a busy port as free — that is what happened
// when wslhost already owned 127.0.0.1:22. A read-only query is reliable and
// does not interfere with Go's own socket options.
func listenTCP(port int) (*net.TCPListener, error) {
	if port != 0 && portInUse(port) {
		return nil, errPortBusy
	}
	return net.ListenTCP("tcp", &net.TCPAddr{Port: port})
}

// Only the IPv4 table is queried: MIB_TCPROW_OWNER_PID has a fixed, known
// layout (24-byte rows, dwLocalPort at offset 8). An IPv6-only listener is
// still caught by the bind itself failing, which triggers the same fallback.
func portInUse(port int) bool {
	return tableHasPort(port, afInet, 24, 8)
}

// tableHasPort scans the local port of every LISTEN row in
// GetExtendedTcpTable(TCP_TABLE_OWNER_PID_LISTENER). Any failure — a missing
// DLL, an unexpected return code, a short or malformed table — is treated as
// "not in use", so a broken query can never force every forward onto a
// random port.
func tableHasPort(port int, family uint32, rowSize, portOff int) bool {
	var size uint32
	r, _, _ := getExtendedTcpTable.Call(
		0, uintptr(unsafe.Pointer(&size)), 0,
		uintptr(family), tcpTableOwnerPIDListener, 0)
	if r != errorInsufficientBuffer || size < 4 {
		return false
	}
	buf := make([]byte, size)
	r, _, _ = getExtendedTcpTable.Call(
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 0,
		uintptr(family), tcpTableOwnerPIDListener, 0)
	if r != 0 {
		return false
	}
	n := int(binary.LittleEndian.Uint32(buf[:4]))
	for i := 0; i < n; i++ {
		off := 4 + i*rowSize
		if off+portOff+2 > len(buf) {
			return false // malformed table: do not guess
		}
		// dwLocalPort holds the port in network byte order in its low 16 bits.
		if int(binary.BigEndian.Uint16(buf[off+portOff:off+portOff+2])) == port {
			return true
		}
	}
	return false
}
