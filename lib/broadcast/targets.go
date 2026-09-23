package broadcast

import (
	"io/ioutil"
	"net"
	"strings"
)

// Targets returns the UDP destinations every broadcast is sent to:
// the global broadcast, per-interface directed broadcasts, and (most
// reliable under WSL2 NAT) a unicast to the Windows host taken from
// resolv.conf. Deduplicated, IPv4 only.
func Targets() []*net.UDPAddr {
	seen := map[string]bool{}
	var out []*net.UDPAddr
	add := func(ip net.IP) {
		if ip == nil {
			return
		}
		ip4 := ip.To4()
		if ip4 == nil {
			return
		}
		key := ip4.String()
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, &net.UDPAddr{IP: ip4, Port: Port})
	}

	add(net.ParseIP("255.255.255.255"))

	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, a := range addrs {
				ipnet, ok := a.(*net.IPNet)
				if !ok {
					continue
				}
				ip4 := ipnet.IP.To4()
				if ip4 == nil || len(ipnet.Mask) != net.IPv4len {
					continue
				}
				bc := make(net.IP, net.IPv4len)
				for i := 0; i < net.IPv4len; i++ {
					bc[i] = ip4[i] | ^ipnet.Mask[i]
				}
				add(bc)
			}
		}
	}

	if host := hostIPFromResolvConf("/etc/resolv.conf"); host != nil {
		add(host)
	}
	return out
}

// hostIPFromResolvConf returns the first nameserver IP.
// Inside WSL this is the Windows host, reachable even when broadcasts are filtered.
func hostIPFromResolvConf(path string) net.IP {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 2 && fields[0] == "nameserver" {
			if ip := net.ParseIP(fields[1]); ip != nil {
				return ip
			}
		}
	}
	return nil
}
