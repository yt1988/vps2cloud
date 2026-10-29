package main

import (
	"bytes"
	"compress/flate"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
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

// A share link carries the whole result in the URL fragment, which browsers never send to a server. Format 2 is
// binary (see docs/share-format.md): regions are bits in a bitmap in endpoint-list order rather than names, and
// numbers are varints, which keeps a full 361-region run near 2,100 characters instead of 7,500. The list is named by a
// fingerprint, and the site keeps every list it has served, so old links still decode after regions are added. A run
// over both IP families carries one block per family. Never the IP.
const shareFormat = 2

const (
	flagPeak = 1 << iota // 20:00–23:00 local
	flagV6               // the only family is IPv6
	flagCLI              // made by this tool
	flagBoth             // two blocks: IPv4, then IPv6
)

var modeCode = map[string]byte{"tcp": 0, "https": 1, "icmp": 2}

// fingerprint names an endpoint list by its region ids in order: the first 6 bytes of their SHA-256.
func fingerprint(l *List) []byte {
	h := sha256.New()
	for _, e := range l.Endpoints {
		h.Write([]byte(e.ID + "\n"))
	}
	return h.Sum(nil)[:6]
}

func fingerprintString(l *List) string { return base64.RawURLEncoding.EncodeToString(fingerprint(l)) }

func encodeShare(secs []Section, l *List, modes []string, n int, o *Origin, at time.Time) []byte {
	var b bytes.Buffer
	uv := func(v uint64) { b.Write(binary.AppendUvarint(nil, v)) }
	str := func(s string) { uv(uint64(len(s))); b.WriteString(s) }
	ms10 := func(v float64) uint64 { return uint64(math.Max(0, math.Round(v*10))) }

	flags := byte(flagCLI)
	if h := at.Hour(); h >= 20 && h < 23 {
		flags |= flagPeak
	}
	if len(secs) > 1 {
		flags |= flagBoth
	} else if secs[0].Fam == 6 {
		flags |= flagV6
	}
	b.Write([]byte{shareFormat, flags, modeCode[modes[0]]})
	uv(uint64(n))
	uv(uint64(at.Unix() / 60))
	b.Write(fingerprint(l))
	var city, cc, org string
	if o != nil {
		city, cc, org = o.City, o.Country, o.Org
	}
	str(city)
	str(cc)
	str(org)

	// one block per family: region count, bitmap of regions with a result, then their numbers in list order
	for _, s := range secs {
		byID := map[string]*ModeResult{}
		for _, r := range s.Res {
			if m := r.Primary(modes); m != nil && m.St.OK > 0 {
				byID[r.E.ID] = m
			}
		}
		uv(uint64(len(l.Endpoints)))
		bitmap := make([]byte, (len(l.Endpoints)+7)/8)
		for i, e := range l.Endpoints {
			if byID[e.ID] != nil {
				bitmap[i/8] |= 1 << (i % 8)
			}
		}
		b.Write(bitmap)
		for _, e := range l.Endpoints {
			m := byID[e.ID]
			if m == nil {
				continue
			}
			st := m.St
			uv(ms10(st.Med))
			uv(ms10(st.P90 - st.Med))
			uv(ms10(st.Jit))
			b.WriteByte(byte(math.Round(st.Loss * 100)))
		}
	}

	var z bytes.Buffer
	w, _ := flate.NewWriter(&z, flate.BestCompression)
	w.Write(b.Bytes())
	w.Close()
	return z.Bytes()
}

func shareLink(secs []Section, l *List, modes []string, n int, o *Origin, at time.Time) string {
	return siteURL() + "/r#s=" + base64.RawURLEncoding.EncodeToString(encodeShare(secs, l, modes, n, o, at))
}

// shortenerURL is the 1ms.dog short link service. It stores the full link (and so the result in it) and returns
// https://s.1ms.dog/<code>, which redirects there; the same link always gets the same code. It only takes links on
// 1ms.dog, so a self-hosted site (V2C_SITE) keeps full links.
var shortenerURL = "https://s.1ms.dog/api/shorten"

func shorten(long, ua string) (string, error) {
	if u, err := url.Parse(long); err != nil || !(u.Hostname() == "1ms.dog" || strings.HasSuffix(u.Hostname(), ".1ms.dog")) {
		return "", errors.New("not a 1ms.dog link")
	}
	body, _ := json.Marshal(map[string]string{"url": long})
	req, _ := http.NewRequest("POST", shortenerURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", ua)
	res, err := (&http.Client{Timeout: 6 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	var out struct {
		URL   string `json:"url"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(&out); err != nil {
		return "", err
	}
	if res.StatusCode != 200 || !strings.HasPrefix(out.URL, "https://") {
		return "", fmt.Errorf("%s %s", res.Status, out.Error)
	}
	return out.URL, nil
}
