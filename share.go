package main

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/json"
	"math"
	"os"
	"strings"
	"time"
)

// siteURL is where share links open; V2C_SITE points them at a self-hosted copy.
func siteURL() string {
	if s := strings.TrimRight(os.Getenv("V2C_SITE"), "/"); s != "" {
		return s
	}
	return "https://v2c.1ms.dog"
}

// A share link carries the whole result in the URL fragment, which browsers never send to a server. The payload is
// Me2Cloud's share format (so the same viewer reads both), plus "m" for the measurement and "c" for the tool:
// per provider [region, median, p25, p75, p90, jitter (ms × 10), loss %, samples]. Never the IP.
func shareLink(res []*Result, modes []string, n, fam int, o *Origin, at time.Time) string {
	r := map[string][][]any{}
	k := func(v float64) int { return int(math.Round(v * 10)) }
	for _, x := range res {
		m := x.Primary(modes)
		if m == nil || m.St.OK == 0 {
			continue
		}
		st := m.St
		r[x.E.Vendor] = append(r[x.E.Vendor], []any{x.E.Region, k(st.Med), k(st.P25), k(st.P75), k(st.P90), k(st.Jit), int(math.Round(st.Loss * 100)), st.N})
	}
	p := map[string]any{"v": 1, "t": at.Unix() / 60, "p": 0, "k": n, "f": fam, "x": 0, "l": 0, "r": r, "m": modes[0], "c": "cli/" + version}
	if h := at.Hour(); h >= 20 && h < 23 {
		p["p"] = 1
	}
	if o != nil && (o.City != "" || o.Org != "") {
		p["l"] = []any{o.City, o.Country, o.Org}
	}
	j, _ := json.Marshal(p)
	var b bytes.Buffer
	w, _ := flate.NewWriter(&b, flate.BestCompression)
	w.Write(j)
	w.Close()
	return siteURL() + "/r#s=" + base64.RawURLEncoding.EncodeToString(b.Bytes())
}
