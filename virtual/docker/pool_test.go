package docker

import (
	"testing"
)

func TestIPPool(t *testing.T) {
	pool := newIPPoolFromHost()

	ips := map[string]struct{}{}
	n := 30000
	for i := 0; i < n; i++ {
		ip, err := pool.Get()
		if err != nil {
			break
		}

		ips[ip] = struct{}{}
	}

	if len(ips) != n {
		t.Fatalf("expected %d unique ip ranges, but received: %d", n, len(ips))
	}
}
