package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Stats match Me2Cloud's: quantiles by linear interpolation over the answered samples, jitter as the mean absolute
// difference between consecutive answered samples, loss as the share of samples without an answer.
type Stats struct {
	N, OK                             int
	Loss                              float64
	Min, P25, Med, P75, P90, Max, Avg float64
	Jit                               float64
}

func calc(s []float64) Stats {
	var ok []float64
	for _, v := range s {
		if !math.IsNaN(v) {
			ok = append(ok, v)
		}
	}
	st := Stats{N: len(s), OK: len(ok)}
	if st.N > 0 {
		st.Loss = float64(st.N-st.OK) / float64(st.N)
	}
	if len(ok) == 0 {
		return st
	}
	for i := 1; i < len(ok); i++ {
		st.Jit += math.Abs(ok[i] - ok[i-1])
	}
	if len(ok) > 1 {
		st.Jit /= float64(len(ok) - 1)
	}
	a := append([]float64(nil), ok...)
	sort.Float64s(a)
	q := func(p float64) float64 {
		i := p * float64(len(a)-1)
		lo, hi := int(math.Floor(i)), int(math.Ceil(i))
		return a[lo] + (a[hi]-a[lo])*(i-float64(lo))
	}
	sum := 0.0
	for _, v := range a {
		sum += v
	}
	st.Min, st.P25, st.Med, st.P75, st.P90, st.Max, st.Avg = a[0], q(.25), q(.5), q(.75), q(.9), a[len(a)-1], sum/float64(len(a))
	return st
}

type ModeResult struct {
	Samples []float64
	St      Stats
	Err     string
	Reconn  int // HTTPS samples that had to open a new connection
}

type Result struct {
	E     Endpoint
	Addr  string
	Err   string // name resolution failed: no mode ran
	Modes map[string]*ModeResult
}

// Primary is the result of the first mode asked for, which ranks the table and goes into the share link.
func (r *Result) Primary(modes []string) *ModeResult { return r.Modes[modes[0]] }

const errNoV6 = "no IPv6"

type Plan struct {
	Probe
	Modes       []string
	Fam         int
	Concurrency int
	VendorCap   int
	ICMPOK      bool
}

// Run measures every endpoint: at most VendorCap hosts of one provider and Concurrency hosts overall at a time, with the
// modes of one host run one after the other.
func (pl Plan) Run(ctx context.Context, eps []Endpoint, progress func(done, total int, e Endpoint)) []*Result {
	out := make([]*Result, len(eps))
	global := make(chan struct{}, pl.Concurrency)
	vendor := map[string]chan struct{}{}
	for _, e := range eps {
		if vendor[e.Vendor] == nil {
			vendor[e.Vendor] = make(chan struct{}, pl.VendorCap)
		}
	}
	var mu sync.Mutex
	done := 0
	var wg sync.WaitGroup
	for i, e := range eps {
		wg.Add(1)
		go func() {
			defer wg.Done()
			vendor[e.Vendor] <- struct{}{}
			global <- struct{}{}
			out[i] = pl.one(ctx, e)
			<-global
			<-vendor[e.Vendor]
			mu.Lock()
			done++
			if progress != nil {
				progress(done, len(eps), e)
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out
}

func (pl Plan) one(ctx context.Context, e Endpoint) *Result {
	r := &Result{E: e, Modes: map[string]*ModeResult{}}
	ip, err := resolve(ctx, e.Host(pl.Fam), pl.Fam)
	if err != nil {
		r.Err = "dns: " + shortErr(err)
		if pl.Fam == 6 {
			r.Err = errNoV6 // most often the host has no IPv6 address at all
		}
		return r
	}
	r.Addr = ip.String()
	for _, m := range pl.Modes {
		mr := &ModeResult{}
		var s []float64
		switch m {
		case "tcp":
			s, err = pl.TCP(ctx, ip, e.Port())
		case "https":
			s, mr.Reconn, err = pl.HTTPS(ctx, e, pl.Fam, ip)
		case "icmp":
			if !pl.ICMPOK {
				mr.Err = "icmp unavailable"
			} else if e.ICMP != nil && !*e.ICMP {
				mr.Err = "host drops icmp"
			} else {
				s, err = pl.ICMP(ctx, ip)
			}
		}
		if s == nil && mr.Err == "" && err != nil {
			mr.Err = shortErr(err)
			s = make([]float64, pl.N)
			for i := range s {
				s[i] = lost
			}
		}
		mr.Samples = s
		mr.St = calc(s)
		if mr.St.OK == 0 && mr.Err == "" && err != nil {
			mr.Err = shortErr(err)
		}
		r.Modes[m] = mr
	}
	return r
}

func shortErr(err error) string {
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return "timeout"
	}
	if oe, ok := err.(*net.OpError); ok {
		return oe.Err.Error()
	}
	s := err.Error()
	if len(s) > 60 {
		s = s[:60]
	}
	return s
}

// Where the measurement runs from, as Cloudflare sees it (over the chosen IP family). The IP stays out of the output
// unless --show-ip is given, and never goes into a share link.
type Origin struct {
	IP      string `json:"ip,omitempty"`
	ASN     int    `json:"asn"`
	Org     string `json:"org"`
	City    string `json:"city"`
	Country string `json:"country"`
	Colo    string `json:"colo"`
}

func origin(fam int, ua string) *Origin {
	network := "tcp4"
	if fam == 6 {
		network = "tcp6"
	}
	tr := &http.Transport{DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 4 * time.Second}).DialContext(ctx, network, addr)
	}}
	cl := &http.Client{Transport: tr, Timeout: 6 * time.Second}
	get := func(u string) (*http.Response, error) {
		req, _ := http.NewRequest("GET", u, nil)
		req.Header.Set("User-Agent", ua)
		req.Header.Set("Referer", "https://speed.cloudflare.com/")
		return cl.Do(req)
	}
	o := &Origin{}
	if res, err := get("https://speed.cloudflare.com/meta"); err == nil {
		var m struct {
			ClientIP       string          `json:"clientIp"`
			ASN            json.RawMessage `json:"asn"`
			AsOrganization string          `json:"asOrganization"`
			City           string          `json:"city"`
			Country        string          `json:"country"`
			Colo           struct {
				IATA string `json:"iata"`
			} `json:"colo"`
		}
		if json.NewDecoder(res.Body).Decode(&m) == nil {
			asn, _ := strconv.Atoi(string(m.ASN))
			o = &Origin{IP: m.ClientIP, ASN: asn, Org: m.AsOrganization, City: m.City, Country: m.Country, Colo: m.Colo.IATA}
		}
		res.Body.Close()
	}
	// /meta comes back empty outside a browser on some networks: ident.me (also used by Me2Cloud) names the network
	if o.Org == "" {
		host := "https://v4.ident.me/json"
		if fam == 6 {
			host = "https://v6.ident.me/json"
		}
		if res, err := get(host); err == nil {
			var m struct {
				IP   string          `json:"ip"`
				ASN  json.RawMessage `json:"asn"`
				ASO  string          `json:"aso"`
				CC   string          `json:"cc"`
				City string          `json:"city"`
			}
			if json.NewDecoder(res.Body).Decode(&m) == nil && m.IP != "" {
				asn, _ := strconv.Atoi(strings.Trim(string(m.ASN), `"`))
				o.IP, o.ASN, o.Org = m.IP, asn, m.ASO
				if o.City == "" {
					o.City, o.Country = m.City, m.CC
				}
			}
			res.Body.Close()
		}
	}
	// and the trace still gives the IP, country and colo
	if o.IP == "" || o.Colo == "" {
		if res, err := get("https://speed.cloudflare.com/cdn-cgi/trace"); err == nil {
			b, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
			res.Body.Close()
			for _, ln := range strings.Split(string(b), "\n") {
				k, v, _ := strings.Cut(ln, "=")
				switch k {
				case "ip":
					o.IP = v
				case "loc":
					o.Country = v
				case "colo":
					o.Colo = v
				}
			}
		}
	}
	if o.IP == "" && o.Country == "" {
		return nil
	}
	return o
}
