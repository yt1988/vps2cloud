package main

import (
	"flag"
	"io"
	"strings"
	"testing"
	"time"
)

func okResult(id, continent, addr string, tcp, https float64) *Result {
	r := &Result{E: Endpoint{ID: id, Vendor: "aws", Region: id, Continent: continent, URL: "https://x/"}, Fam: 4, Addr: addr, Modes: map[string]*ModeResult{}}
	mk := func(v float64) *ModeResult {
		s := []float64{v, v, v}
		return &ModeResult{Samples: s, St: calc(s)}
	}
	r.Modes["tcp"] = mk(tcp)
	if https > 0 {
		r.Modes["https"] = mk(https)
	}
	return r
}

func TestProxySigns(t *testing.T) {
	tcp := []string{"tcp"}
	// a fake-IP address gives the sign away on its own, whatever the numbers
	if s, d := proxySigns([]*Result{okResult("a", "eu", "198.19.77.47", 120, 0)}, tcp); s != "fake" || d != "198.19.77.47" {
		t.Fatalf("fake: got %q %q", s, d)
	}
	for _, a := range []string{"10.1.2.3", "192.168.1.1", "127.0.0.1", "fc00::1", "::ffff:172.16.0.9"} {
		if s, _ := proxySigns([]*Result{okResult("a", "eu", a, 120, 0)}, tcp); s != "fake" {
			t.Errorf("%s: want fake, got %q", a, s)
		}
	}
	// three continents all under 5 ms cannot be real
	fast := []*Result{okResult("a", "eu", "3.3.3.3", 0.5, 0), okResult("b", "na", "3.3.3.4", 0.6, 0), okResult("c", "as", "3.3.3.5", 0.5, 0),
		okResult("d", "eu", "3.3.3.6", 1.1, 0), okResult("e", "sa", "3.3.3.7", 0.9, 0)}
	if s, _ := proxySigns(fast, tcp); s != "fast" {
		t.Fatalf("fast: got %q", s)
	}
	// the same with one honest region is a real network near a hub, not a proxy
	fast[3] = okResult("d", "eu", "3.3.3.6", 140, 0)
	if s, _ := proxySigns(fast, tcp); s != "" {
		t.Fatalf("one slow region: got %q", s)
	}
	// HTTPS far slower than the handshake, on most regions: the handshake was local
	gap := []*Result{okResult("a", "eu", "3.3.3.3", 0.5, 130), okResult("b", "eu", "3.3.3.4", 0.6, 180), okResult("c", "eu", "3.3.3.5", 0.5, 2)}
	if s, _ := proxySigns(gap, []string{"tcp", "https"}); s != "gap" {
		t.Fatalf("gap: got %q", s)
	}
	// a normal run from Nuremberg: nothing to say
	normal := []*Result{okResult("a", "eu", "3.3.3.3", 0.7, 2), okResult("b", "na", "3.3.3.4", 95, 110), okResult("c", "as", "3.3.3.5", 240, 260)}
	if s, _ := proxySigns(normal, []string{"tcp", "https"}); s != "" {
		t.Fatalf("normal: got %q", s)
	}
	if s, _ := proxySigns(nil, tcp); s != "" {
		t.Fatalf("empty: got %q", s)
	}
}

func TestProxyFlagInShareLink(t *testing.T) {
	l := &List{Schema: 1, Vendors: map[string]Vendor{"aws": {Name: "AWS"}}, Endpoints: []Endpoint{{ID: "aws:a", Vendor: "aws", Region: "a", URL: "https://a/"}}}
	res := []*Result{okResult("aws:a", "eu", "198.18.0.5", 0.5, 0)}
	res[0].E = l.Endpoints[0]
	with := decodeShare(t, shareLink([]Section{{Fam: 4, Res: res}}, l, []string{"tcp"}, 3, nil, time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC), true))
	without := decodeShare(t, shareLink([]Section{{Fam: 4, Res: res}}, l, []string{"tcp"}, 3, nil, time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC), false))
	if with.flags&flagProxy == 0 || without.flags&flagProxy != 0 {
		t.Fatalf("proxy flag: with=%08b without=%08b", with.flags, without.flags)
	}
}

func TestFlagError(t *testing.T) {
	lang = "en"
	fs := flag.NewFlagSet("x", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	for _, n := range []string{"p", "provider", "continent", "country", "count", "concurrency", "mode", "no-link"} {
		fs.String(n, "", "")
	}
	cases := map[string]string{"--cont": "continent", "--contient": "continent", "--provdier": "provider", "--nolink": "no-link", "--zzz": ""}
	for arg, want := range cases {
		err := fs.Parse([]string{arg, "x"})
		if err == nil {
			t.Fatalf("%s parsed", arg)
		}
		got := flagError(fs, err)
		if !strings.Contains(got, "no option "+arg) {
			t.Errorf("%s: %q lacks the option name", arg, got)
		}
		if want != "" && !strings.Contains(got, "mean --"+want+"?") {
			t.Errorf("%s: %q does not suggest --%s", arg, got, want)
		}
		if want == "" && strings.Contains(got, "mean") {
			t.Errorf("%s: %q suggests something for nonsense", arg, got)
		}
	}
	// a value the flag cannot read keeps the flag package's words, plus the pointer to --help
	fs.Int("n", 0, "")
	if got := flagError(fs, fs.Parse([]string{"-n", "many"})); !strings.Contains(got, "--help") || !strings.Contains(got, "many") {
		t.Errorf("bad value: %q", got)
	}
}

func TestLangArg(t *testing.T) {
	for _, c := range [][2]string{{"--lang zh-CN -p aws", "zh-CN"}, {"-p aws --lang=ja", "ja"}, {"-p aws", ""}, {"--lang", ""}} {
		if got := langArg(strings.Fields(c[0])); got != c[1] {
			t.Errorf("%q: got %q, want %q", c[0], got, c[1])
		}
	}
}

func TestFormatSmall(t *testing.T) {
	if f1(0.0) != "<0.1" || f1(0.04) != "<0.1" || f1(0.05) != "0.1" || f1(0.5) != "0.5" || f1(120.4) != "120" {
		t.Fatalf("f1: %s %s %s %s %s", f1(0), f1(0.04), f1(0.05), f1(0.5), f1(120.4))
	}
	if jit(0) != "±0" || jit(0.3) != "±0.3" {
		t.Fatalf("jit: %s %s", jit(0), jit(0.3))
	}
	if cityText("?") != "" || cityText("--") != "" || cityText("Tokyo") != "Tokyo" {
		t.Fatal("cityText")
	}
	if regionText(Endpoint{ID: "cdn:fastly", Region: "edge"}) != "" || regionText(Endpoint{ID: "aws:x", Region: "x"}) != "x" {
		t.Fatal("regionText")
	}
}
