//go:build windows
// +build windows

package proxy

import (
	"context"
	"fmt"
	"net"
	"syscall"
)

// SO_EXCLUSIVEADDRUSE (windows.h) is ~SO_REUSEADDR. Without it Windows lets a
// wildcard bind (0.0.0.0:P) coexist with an existing specific-IP bind
// (127.0.0.1:P), so a plain net.ListenTCP would report a busy port as free —
// exactly what happened when a host service already owned port 22.
const soExclusiveAddrUse = -5

// listenTCP binds a TCP port exclusively, so any existing listener on that
// port (wildcard or specific IP) makes the bind fail and the caller falls
// back to a free ephemeral port.
func listenTCP(port int) (*net.TCPListener, error) {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var optErr error
			if err := c.Control(func(fd uintptr) {
				optErr = syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, soExclusiveAddrUse, 1)
			}); err != nil {
				return err
			}
			return optErr
		},
	}
	ln, err := lc.Listen(context.Background(), "tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	tcp, ok := ln.(*net.TCPListener)
	if !ok {
		ln.Close()
		return nil, fmt.Errorf("unexpected listener type %T", ln)
	}
	return tcp, nil
}
