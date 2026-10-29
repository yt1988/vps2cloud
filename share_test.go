package main

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/binary"
	"io"
	"math/rand/v2"
	"strings"
	"testing"
	"time"
)

type decoded struct {
	flags, mode   byte
	n, minutes    uint64
	fp            []byte
	city, cc, org string
	count         uint64
	rows          map[int][4]uint64 // list index → median×10, (p90−median)×10, jitter×10, loss %
}

// decodeShare follows docs/share-format.md, as a viewer would.
func decodeShare(t *testing.T, link string) decoded {
	t.Helper()
	data, err := base64.RawURLEncoding.DecodeString(link[strings.Index(link, "#s=")+3:])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(flate.NewReader(bytes.NewReader(data)))
	if err != nil {
		t.Fatal(err)
	}
	r := bytes.NewReader(raw)
	byte1 := func() byte { b, _ := r.ReadByte(); return b }
	uv := func() uint64 {
		v, err := binary.ReadUvarint(r)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	str := func() string { b := make([]byte, uv()); io.ReadFull(r, b); return string(b) }
	if v := byte1(); v != 2 {
		t.Fatalf("format %d", v)
	}
	d := decoded{flags: byte1(), mode: byte1(), rows: map[int][4]uint64{}}
	d.n, d.minutes = uv(), uv()
	d.fp = make([]byte, 6)
	io.ReadFull(r, d.fp)
	d.city, d.cc, d.org = str(), str(), str()
	d.count = uv()
	bitmap := make([]byte, (d.count+7)/8)
	io.ReadFull(r, bitmap)
	for i := 0; i < int(d.count); i++ {
		if bitmap[i/8]&(1<<(i%8)) != 0 {
			d.rows[i] = [4]uint64{uv(), uv(), uv(), uint64(byte1())}
		}
	}
	if r.Len() != 0 {
		t.Fatalf("%d bytes left over", r.Len())
	}
	return d
}

func fakeResults(l *List, every int) []*Result {
	rng := rand.New(rand.NewPCG(1, 2))
	var res []*Result
	for i, e := range l.Endpoints {
		if i%every != 0 {
			continue
		}
		med := 1 + rng.Float64()*350
		res = append(res, &Result{E: e, Modes: map[string]*ModeResult{"tcp": {St: Stats{N: 10, OK: 9, Med: med, P90: med * 1.1, Jit: rng.Float64() * 20, Loss: 0.1}}}})
	}
	return res
}

func TestShareRoundTrip(t *testing.T) {
	l, _ := parseList(embeddedList)
	res := fakeResults(l, 3)
	at := time.Date(2026, 10, 29, 21, 5, 0, 0, time.Local)
	link := shareLink(res, l, []string{"tcp"}, 10, 6, &Origin{City: "Nürnberg", Country: "DE", Org: "netcup GmbH", IP: "203.0.113.9"}, at)
	if strings.Contains(link, "203.0.113.9") {
		t.Fatal("IP in link")
	}
	d := decodeShare(t, link)
	if d.flags != 1|2|4 || d.mode != 0 || d.n != 10 || d.minutes != uint64(at.Unix()/60) {
		t.Errorf("header: %+v", d)
	}
	if !bytes.Equal(d.fp, fingerprint(l)) || d.count != uint64(len(l.Endpoints)) {
		t.Errorf("list: %x %d", d.fp, d.count)
	}
	if d.city != "Nürnberg" || d.cc != "DE" || d.org != "netcup GmbH" {
		t.Errorf("from: %q %q %q", d.city, d.cc, d.org)
	}
	if len(d.rows) != len(res) {
		t.Fatalf("%d rows, want %d", len(d.rows), len(res))
	}
	for _, r := range res {
		idx := -1
		for i, e := range l.Endpoints {
			if e.ID == r.E.ID {
				idx = i
			}
		}
		st, got := r.Modes["tcp"].St, d.rows[idx]
		if want := ms10(st.Med); got[0] != want {
			t.Errorf("%s median %d want %d", r.E.ID, got[0], want)
		}
		if got[3] != 10 {
			t.Errorf("%s loss %d", r.E.ID, got[3])
		}
	}
}

func ms10(v float64) uint64 { return uint64(v*10 + 0.5) }

func TestShareLength(t *testing.T) {
	l, _ := parseList(embeddedList)
	link := shareLink(fakeResults(l, 1), l, []string{"tcp"}, 10, 4, &Origin{City: "Nuremberg", Country: "DE", Org: "netcup GmbH"}, time.Now())
	t.Logf("all %d regions: %d characters", len(l.Endpoints), len(link))
	if len(link) > 3000 {
		t.Errorf("link is %d characters", len(link))
	}
	link = shareLink(fakeResults(l, 18), l, []string{"tcp"}, 10, 4, nil, time.Now())
	t.Logf("20 regions: %d characters", len(link))
}
