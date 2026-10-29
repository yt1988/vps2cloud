package main

import (
	"bytes"
	"compress/flate"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
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

// A share link carries the whole result in the URL fragment, which browsers never send to a server. Format 2 is
// binary (see docs/share-format.md): regions are bits in a bitmap in endpoint-list order rather than names, and
// numbers are varints, which keeps a full 361-region run near 2,100 characters instead of 7,500. The list is named by a
// fingerprint, and the site keeps every list it has served, so old links still decode after regions are added.
// Never the IP.
const shareFormat = 2

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

func encodeShare(res []*Result, l *List, modes []string, n, fam int, o *Origin, at time.Time) []byte {
	var b bytes.Buffer
	uv := func(v uint64) { b.Write(binary.AppendUvarint(nil, v)) }
	str := func(s string) { uv(uint64(len(s))); b.WriteString(s) }
	ms10 := func(v float64) uint64 { return uint64(math.Max(0, math.Round(v*10))) }

	flags := byte(4) // from the CLI
	if h := at.Hour(); h >= 20 && h < 23 {
		flags |= 1
	}
	if fam == 6 {
		flags |= 2
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

	byID := map[string]*ModeResult{}
	for _, r := range res {
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

	var z bytes.Buffer
	w, _ := flate.NewWriter(&z, flate.BestCompression)
	w.Write(b.Bytes())
	w.Close()
	return z.Bytes()
}

func shareLink(res []*Result, l *List, modes []string, n, fam int, o *Origin, at time.Time) string {
	return siteURL() + "/r#s=" + base64.RawURLEncoding.EncodeToString(encodeShare(res, l, modes, n, fam, o, at))
}
