package proxy

import "net"

// localProbeIPs are the local IPv4 addresses checked for an existing listener.
// 127.0.0.1 is where WSL2's wslhost.exe forwards WSL ports; 0.0.0.0 covers a
// wildcard listener; interface addresses cover services bound to one NIC.
// Windows lets a wildcard bind coexist with a loopback or specific-IP listener
// on the same port, so no single bind is enough: every address has to be free.
func localProbeIPs() []net.IP {
	ips := []net.IP{net.IPv4(127, 0, 0, 1), net.IPv4zero}
	seen := map[string]bool{"127.0.0.1": true, "0.0.0.0": true}
	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	for _, ifi := range ifaces {
		addrs, err := ifi.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip4 := ip.To4(); ip4 != nil && !seen[ip4.String()] {
				seen[ip4.String()] = true
				ips = append(ips, ip4)
			}
		}
	}
	return ips
}

// portOccupied reports whether any local IPv4 address already has a TCP
// listener on port. The rule is deliberately strict: the WSL port is reused
// only when it is completely free on every local address, otherwise the
// forward goes to a random port. This keeps the decision unambiguous instead
// of relying on Windows' wildcard-versus-loopback coexistence rules.
//
// It uses only plain Go socket calls — no OS-specific tables, socket options
// or external commands — so it behaves the same on every platform.
func portOccupied(port int) bool {
	for _, ip := range localProbeIPs() {
		// A down or virtual interface cannot be bound at all. Verify the
		// address is usable with an ephemeral port first, so an unusable
		// address is never mistaken for an occupied one.
		probe, err := net.ListenTCP("tcp", &net.TCPAddr{IP: ip, Port: 0})
		if err != nil {
			continue
		}
		_ = probe.Close()

		ln, err := net.ListenTCP("tcp", &net.TCPAddr{IP: ip, Port: port})
		if err != nil {
			return true
		}
		_ = ln.Close()
	}
	return false
}
