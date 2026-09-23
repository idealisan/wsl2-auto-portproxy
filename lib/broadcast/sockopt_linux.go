//go:build linux
// +build linux

package broadcast

import (
	"net"
	"syscall"
)

// SetBroadcast enables SO_BROADCAST on a UDP socket so it may send to
// broadcast addresses (255.255.255.255 and subnet directed broadcasts).
func SetBroadcast(pc net.PacketConn) error {
	uc, ok := pc.(*net.UDPConn)
	if !ok {
		return nil
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return err
	}
	var optErr error
	if err := raw.Control(func(fd uintptr) {
		optErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
	}); err != nil {
		return err
	}
	return optErr
}
