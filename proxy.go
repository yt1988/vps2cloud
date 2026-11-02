package main

import "net/netip"

// A local proxy or TUN (Clash, sing-box, Surge, a VPN client in "fake IP" mode) completes TCP handshakes itself, so
// every region looks a millisecond away and the table is green from top to bottom. Me2Cloud warns about WARP and
// iCloud Private Relay in the browser; this is the same warning for the command line. Three signs, any one is enough:
//
//   - fake: a host resolved to an address no cloud region has: 198.18.0.0/15 (the benchmarking range fake-IP modes
//     hand out), a private or loopback address, or fc00::/7
//   - fast: regions on three or more continents all answered under 5 ms, which no network can do
//   - gap: HTTPS (a request over the proxy's real connection) is over 50× slower than the TCP handshake
//
// proxySigns returns the sign and, for a fake address, the address itself; "" when nothing looks wrong.
func proxySigns(res []*Result, modes []string) (sign, detail string) {
	if len(res) == 0 {
		return "", ""
	}
	for _, r := range res {
		if r.Addr != "" && fakeAddr(r.Addr) {
			return "fake", r.Addr
		}
	}
	continents := map[string]bool{}
	answered, slow := 0, false
	bothN, gapN := 0, 0
	for _, r := range res {
		if r == nil || isCDN(r.E) {
			continue
		}
		p := r.Primary(modes)
		if p == nil || p.St.OK == 0 {
			continue
		}
		answered++
		continents[r.E.Continent] = true
		if p.St.Med >= 5 {
			slow = true
		}
		t, h := r.Modes["tcp"], r.Modes["https"]
		if t != nil && h != nil && t.St.OK > 0 && h.St.OK > 0 {
			bothN++
			if t.St.Med < 5 && h.St.Med > 50*t.St.Med {
				gapN++
			}
		}
	}
	if answered >= 5 && len(continents) >= 3 && !slow {
		return "fast", ""
	}
	if bothN >= 3 && gapN*2 >= bothN {
		return "gap", ""
	}
	return "", ""
}

var fakeRanges = []netip.Prefix{
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("::1/128"),
}

func fakeAddr(s string) bool {
	ip, err := netip.ParseAddr(s)
	if err != nil {
		return false
	}
	ip = ip.Unmap()
	for _, p := range fakeRanges {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// proxyText is the warning line for a sign, in the current language.
func proxyText(sign, detail string) string {
	var why string
	switch sign {
	case "fake":
		why = T("proxyFake", detail)
	case "fast":
		why = T("proxyFast")
	case "gap":
		why = T("proxyGap")
	default:
		return ""
	}
	return T("proxyWarn", why)
}
