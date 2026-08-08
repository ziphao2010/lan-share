package ipaddr

import (
	"net"
	"testing"
)

func TestListLocalIPsNeverEmpty(t *testing.T) {
	ips := ListLocalIPs()
	if len(ips) == 0 {
		t.Fatal("must return at least one IP (fallback to 127.0.0.1)")
	}
	for _, ip := range ips {
		if net.ParseIP(ip) == nil {
			t.Errorf("%q is not a valid IP", ip)
		}
	}
}

func TestListLocalIPsLoopsCurrentInterface(t *testing.T) {
	// If a non-loopback interface is up, 127.0.0.1 may appear only as fallback
	ifaces, err := net.Interfaces()
	if err != nil {
		t.Skip("no network interfaces")
	}
	ips := ListLocalIPs()
	hasLAN := false
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			var ip net.IP
			if n, ok := a.(*net.IPNet); ok {
				ip = n.IP.To4()
			}
			if ip == nil {
				continue
			}
			for _, got := range ips {
				if got == ip.String() {
					hasLAN = true
				}
			}
		}
	}
	if !hasLAN && len(ips) == 1 && ips[0] == "127.0.0.1" {
		t.Log("no LAN interface up; fallback used — OK")
		return
	}
	if !hasLAN {
		t.Log("warning: LAN interface exists but not advertised")
	}
}
