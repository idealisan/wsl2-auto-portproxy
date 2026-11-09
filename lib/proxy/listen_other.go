//go:build !windows
// +build !windows

package proxy

import "net"

// On Unix a wildcard bind already fails with EADDRINUSE when any specific-IP
// listener owns the port, so no extra socket option is needed.
func listenTCP(port int) (*net.TCPListener, error) {
	return net.ListenTCP("tcp", &net.TCPAddr{Port: port})
}
