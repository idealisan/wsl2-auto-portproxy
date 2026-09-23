//go:build !linux && !windows
// +build !linux,!windows

package broadcast

import (
	"net"
)

// SetBroadcast is a no-op on platforms where the agent never broadcasts.
func SetBroadcast(pc net.PacketConn) error {
	return nil
}
