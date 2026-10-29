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
	blocks        []map[int][4]uint64 // per IP family: list index → median×10, (p90−median)×10, jitter×10, loss %
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
	d := decoded{flags: byte1(), mode: byte1()}
	d.n, d.minutes = uv(), uv()
	d.fp = make([]byte, 6)
	io.ReadFull(r, d.fp)
	d.city, d.cc, d.org = str(), str(), str()
	nblocks := 1
	if d.flags&flagBoth != 0 {
		nblocks = 2
	}
	for k := 0; k < nblocks; k++ {
		count := uv()
		bitmap := make([]byte, (count+7)/8)
		io.ReadFull(r, bitmap)
		rows := map[int][4]uint64{}
		for i := 0; i < int(count); i++ {
			if bitmap[i/8]&(1<<(i%8)) != 0 {
				rows[i] = [4]uint64{uv(), uv(), uv(), uint64(byte1())}
			}
		}
		d.blocks = append(d.blocks, rows)
	}
	if r.Len() != 0 {
		t.Fatalf("%d bytes left over", r.Len())
	}
	return d
}

func fakeResults(l *List, every, fam int) []*Result {
	rng := rand.New(rand.NewPCG(uint64(fam), 2))
	var res []*Result
	for i, e := range l.Endpoints {
		if i%every != 0 {
			continue
		}
		med := 1 + rng.Float64()*350
		res = append(res, &Result{E: e, Fam: fam, Modes: map[string]*ModeResult{"tcp": {St: Stats{N: 10, OK: 9, Med: med, P90: med * 1.1, Jit: rng.Float64() * 20, Loss: 0.1}}}})
	}
	return res
}

func ms10(v float64) uint64 { return uint64(v*10 + 0.5) }

func indexOf(l *List, id string) int {
	for i, e := range l.Endpoints {
		if e.ID == id {
			return i
		}
	}
	return -1
}

func TestShareRoundTrip(t *testing.T) {
	l, _ := parseList(embeddedList)
	res := fakeResults(l, 3, 6)
	at := time.Date(2026, 10, 29, 21, 5, 0, 0, time.Local)
	link := shareLink([]Section{{Fam: 6, Res: res}}, l, []string{"tcp"}, 10, &Origin{City: "Nürnberg", Country: "DE", Org: "netcup GmbH", IP: "203.0.113.9"}, at)
	if strings.Contains(link, "203.0.113.9") {
		t.Fatal("IP in link")
	}
	d := decodeShare(t, link)
	if d.flags != flagPeak|flagV6|flagCLI || d.mode != 0 || d.n != 10 || d.minutes != uint64(at.Unix()/60) {
		t.Errorf("header: %+v", d)
	}
	if !bytes.Equal(d.fp, fingerprint(l)) {
		t.Errorf("list: %x", d.fp)
	}
	if d.city != "Nürnberg" || d.cc != "DE" || d.org != "netcup GmbH" {
		t.Errorf("from: %q %q %q", d.city, d.cc, d.org)
	}
	if len(d.blocks) != 1 || len(d.blocks[0]) != len(res) {
		t.Fatalf("blocks %d", len(d.blocks))
	}
	for _, r := range res {
		st, got := r.Modes["tcp"].St, d.blocks[0][indexOf(l, r.E.ID)]
		if got[0] != ms10(st.Med) || got[3] != 10 {
			t.Errorf("%s: %v", r.E.ID, got)
		}
	}
}

func TestShareBothFamilies(t *testing.T) {
	l, _ := parseList(embeddedList)
	v4, v6 := fakeResults(l, 2, 4), fakeResults(l, 7, 6)
	d := decodeShare(t, shareLink([]Section{{Fam: 4, Res: v4}, {Fam: 6, Res: v6}}, l, []string{"tcp"}, 5, nil, time.Date(2026, 10, 29, 9, 0, 0, 0, time.Local)))
	if d.flags != flagCLI|flagBoth || len(d.blocks) != 2 || len(d.blocks[0]) != len(v4) || len(d.blocks[1]) != len(v6) {
		t.Fatalf("flags %b, blocks %d", d.flags, len(d.blocks))
	}
	for k, res := range [][]*Result{v4, v6} {
		for _, r := range res {
			if got := d.blocks[k][indexOf(l, r.E.ID)]; got[0] != ms10(r.Modes["tcp"].St.Med) {
				t.Errorf("block %d %s: %v", k, r.E.ID, got)
			}
		}
	}
}

func TestShareLength(t *testing.T) {
	l, _ := parseList(embeddedList)
	o := &Origin{City: "Nuremberg", Country: "DE", Org: "netcup GmbH"}
	link := shareLink([]Section{{Fam: 4, Res: fakeResults(l, 1, 4)}}, l, []string{"tcp"}, 10, o, time.Now())
	t.Logf("all %d regions: %d characters", len(l.Endpoints), len(link))
	if len(link) > 3000 {
		t.Errorf("link is %d characters", len(link))
	}
	both := shareLink([]Section{{Fam: 4, Res: fakeResults(l, 1, 4)}, {Fam: 6, Res: fakeResults(l, 4, 6)}}, l, []string{"tcp"}, 10, o, time.Now())
	t.Logf("all regions over IPv4 and IPv6: %d characters", len(both))
}
