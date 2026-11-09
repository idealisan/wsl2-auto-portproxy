package storage

import (
	"sync"

	"github.com/HobaiRiku/wsl2-auto-portproxy/lib/proxy"
)

// PoolMu guards ProxyPool. The headless wslpp.exe is its only writer, but
// wslpp-gui manages the same pool from a background reconcile goroutine.
var PoolMu sync.Mutex

var ProxyPool []proxy.Proxy
