//go:build windows
// +build windows

package broadcast

import (
	"net"
	"syscall"
)

// SetBroadcast enables SO_BROADCAST on a UDP socket.
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
		optErr = syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
	}); err != nil {
		return err
	}
	return optErr
}
