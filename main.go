// VPS2Cloud measures latency, jitter and loss from the machine it runs on to the regions of 17 cloud providers.
// It uses Me2Cloud's endpoint list (github.com/yt1988/me2cloud) and needs nothing but this binary.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strings"
	"time"
)

var version = "dev" // set at build time

// Hosts of one provider measured at once. Each is a separate regional server and still gets one sample at a time with
// a pause in between, so four at once is as gentle per host as Me2Cloud (two) while a full run takes half as long.
const vendorCap = 4

const usage = `VPS2Cloud: latency from this machine to 361 regions of 17 cloud providers.

Usage: vps2cloud [options]

What to test
  -p, --provider LIST     providers, comma separated (aws,gcp,hetzner …; --list shows all)
  -r, --region LIST       regions by code, id or city, wildcards allowed ("ap-*", "*tokyo*")
      --continent LIST    as, eu, na, sa, oc, me, af
      --country LIST      ISO codes of the region's country (jp,de,us …)
      --near N            quick pass over everything, then measure only the N closest regions

How to measure
  -m, --mode MODE         tcp (default), https, icmp, or all (comma lists work too: tcp,icmp)
  -n, --count N           samples per region (default 10)
  -4 / -6                 IPv4 (default) or IPv6
  -t, --timeout DUR       per sample (default 2s)
      --gap DUR           pause between samples to one host (default 250ms)
  -C, --concurrency N     hosts measured at once (default 24; at most 2 per provider)

Output
  -o, --output FORMAT     table (default), md, json, csv
      --sort KEY          median (default), min, p90, jitter, loss, name
      --top N             only the N best regions (md defaults to 20)
      --preset NAME       mark regions that fit: game, call, ssh, web
      --no-link           don't print the share link
      --show-ip           include this machine's public IP (never in share links)
      --no-color          plain text (also with NO_COLOR set)
  -q, --quiet             no progress line
      --max-ms N          exit with status 3 when the best median is above N ms

Endpoint list
      --offline           use the list built into this binary, no download
      --endpoints FILE    use a local list (Me2Cloud's endpoints.json format)
      --list              show providers and regions, then exit
  -v, --version

Each run fetches the latest endpoint list from v2c.1ms.dog. That request is counted (tool version, OS and CPU type,
whether it runs in CI) and is the only one made besides the measurement itself and Cloudflare's /meta for the
"from" line. --offline skips the download. Results are never uploaded; a share link carries them in the URL.
`

type opts struct {
	providers, regions, continents, countries             string
	mode, output, sortBy, preset, endpoints, timeout, gap string
	n, near, conc, top                                    int
	v4, v6, noLink, showIP, noColor, quiet, offline, list bool
	maxMS                                                 float64
	ver                                                   bool
}

func main() {
	// net/http logs HTTP/2 frames it does not handle (ORIGIN frames from Cloudflare and Google); none matter here
	log.SetOutput(io.Discard)
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	var o opts
	fs := flag.NewFlagSet("vps2cloud", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }
	str := func(p *string, def string, names ...string) {
		for _, n := range names {
			fs.StringVar(p, n, def, "")
		}
	}
	num := func(p *int, def int, names ...string) {
		for _, n := range names {
			fs.IntVar(p, n, def, "")
		}
	}
	flg := func(p *bool, names ...string) {
		for _, n := range names {
			fs.BoolVar(p, n, false, "")
		}
	}
	str(&o.providers, "", "p", "provider", "providers")
	str(&o.regions, "", "r", "region", "regions")
	str(&o.continents, "", "continent")
	str(&o.countries, "", "country")
	str(&o.mode, "tcp", "m", "mode")
	str(&o.output, "table", "o", "output")
	str(&o.sortBy, "median", "sort")
	str(&o.preset, "", "preset")
	str(&o.endpoints, "", "endpoints")
	str(&o.timeout, "2s", "t", "timeout")
	str(&o.gap, "250ms", "gap")
	num(&o.n, 10, "n", "count")
	num(&o.near, 0, "near")
	num(&o.conc, 24, "C", "concurrency")
	num(&o.top, -1, "top")
	flg(&o.v4, "4")
	flg(&o.v6, "6")
	flg(&o.noLink, "no-link")
	flg(&o.showIP, "show-ip")
	flg(&o.noColor, "no-color")
	flg(&o.quiet, "q", "quiet")
	flg(&o.offline, "offline")
	flg(&o.list, "list")
	flg(&o.ver, "v", "version")
	fs.Float64Var(&o.maxMS, "max-ms", 0, "")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument %q (see --help)\n", fs.Arg(0))
		return 2
	}
	if o.ver {
		fmt.Fprintf(stdout, "vps2cloud %s %s/%s\n", version, runtime.GOOS, runtime.GOARCH)
		return 0
	}
	fail := func(format string, a ...any) int { fmt.Fprintf(stderr, "vps2cloud: "+format+"\n", a...); return 2 }

	modes, err := parseModes(o.mode)
	if err != nil {
		return fail("%v", err)
	}
	if o.preset != "" {
		if _, ok := presets[o.preset]; !ok {
			return fail("unknown preset %q (game, call, ssh, web)", o.preset)
		}
	}
	switch o.output {
	case "table", "md", "json", "csv":
	default:
		return fail("unknown output %q (table, md, json, csv)", o.output)
	}
	switch o.sortBy {
	case "median", "min", "p90", "jitter", "loss", "name":
	default:
		return fail("unknown sort key %q", o.sortBy)
	}
	timeout, err1 := time.ParseDuration(o.timeout)
	gap, err2 := time.ParseDuration(o.gap)
	if err1 != nil || err2 != nil || timeout <= 0 || gap < 0 {
		return fail("bad --timeout or --gap (use values like 2s, 250ms)")
	}
	if o.n < 1 || o.n > 100 || o.conc < 1 {
		return fail("--count must be 1–100 and --concurrency at least 1")
	}
	if o.v4 && o.v6 {
		return fail("pick one of -4 and -6")
	}
	fam := 4
	if o.v6 {
		fam = 6
	}

	ua := userAgent()
	list, err := loadList(o.endpoints, o.offline, ua)
	if err != nil {
		fmt.Fprintf(stderr, "vps2cloud: endpoint list: %v\n", err)
		return 1
	}
	if o.list {
		printList(stdout, list, splitList(o.providers))
		return 0
	}
	eps, err := list.Select(Filter{splitList(o.providers), splitList(o.regions), splitList(o.continents), splitList(o.countries)})
	if err != nil {
		return fail("%v", err)
	}
	if len(eps) == 0 {
		return fail("no region matches those filters")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	color := !o.noColor && os.Getenv("NO_COLOR") == "" && isTTY(stdout)
	showProgress := !o.quiet && isTTY(stderr)
	pl := Plan{Probe: Probe{N: o.n, Gap: gap, Timeout: timeout, UA: ua}, Modes: modes, Fam: fam, Concurrency: o.conc, VendorCap: vendorCap}

	if contains(modes, "icmp") {
		if c, _, err := icmpListen(fam); err == nil {
			c.Close()
			pl.ICMPOK = true
		} else if len(modes) == 1 {
			return fail("ICMP needs root, CAP_NET_RAW (setcap cap_net_raw+ep vps2cloud) or a ping_group_range that covers you: %v", err)
		} else if !o.quiet {
			fmt.Fprintln(stderr, "vps2cloud: ICMP not available here (needs root or CAP_NET_RAW); measuring the other modes")
		}
	}

	start := time.Now()
	orig := make(chan *Origin, 1)
	go func() { orig <- origin(fam, ua) }()
	progress := func(phase string) func(done, total int, e Endpoint) {
		if !showProgress {
			return nil
		}
		return func(done, total int, e Endpoint) {
			fmt.Fprintf(stderr, "\r\x1b[K%s %d/%d  %s", phase, done, total, e.ID)
		}
	}

	if o.near > 0 && o.near < len(eps) {
		// a short TCP pass over everything picks the closest regions for the full measurement
		quick := Plan{Probe: Probe{N: 2, Gap: gap, Timeout: timeout, UA: ua}, Modes: []string{"tcp"}, Fam: fam, Concurrency: o.conc, VendorCap: vendorCap}
		first := quick.Run(ctx, eps, progress("scanning"))
		sortResults(first, quick.Modes, "min")
		eps = eps[:0]
		for _, r := range first[:o.near] {
			if p := r.Primary(quick.Modes); p != nil && p.St.OK > 0 {
				eps = append(eps, r.E)
			}
		}
		if len(eps) == 0 {
			if showProgress {
				fmt.Fprint(stderr, "\r\x1b[K")
			}
			fmt.Fprintln(stderr, "vps2cloud: no region answered the quick pass")
			return 1
		}
	}
	res := pl.Run(ctx, eps, progress("measuring"))
	if showProgress {
		fmt.Fprint(stderr, "\r\x1b[K")
	}
	if ctx.Err() != nil {
		fmt.Fprintln(stderr, "vps2cloud: interrupted")
		return 130
	}
	// with -6, regions without an IPv6 address are left out and counted instead of listed as failures
	noV6 := 0
	if fam == 6 {
		kept := res[:0]
		for _, r := range res {
			if r.Err == errNoV6 {
				noV6++
				continue
			}
			kept = append(kept, r)
		}
		res = kept
	}
	sortResults(res, modes, o.sortBy)

	v := View{Res: res, Modes: modes, List: list, Origin: <-orig, ShowIP: o.showIP, Fam: fam, N: o.n, At: start, Took: time.Since(start),
		Preset: o.preset, Top: o.top, Color: color, SortBy: o.sortBy, NoV6: noV6}
	if v.Top < 0 {
		v.Top = 0
		if o.output == "md" {
			v.Top = 20
		}
	}
	if !o.noLink {
		v.Link = shareLink(res, modes, o.n, fam, v.Origin, start)
	}
	switch o.output {
	case "md":
		v.Markdown(stdout)
	case "json":
		v.JSON(stdout)
	case "csv":
		v.CSV(stdout)
	default:
		v.Table(stdout)
	}
	if o.maxMS > 0 {
		best := -1.0
		for _, r := range res {
			if p := r.Primary(modes); p != nil && p.St.OK > 0 && (best < 0 || p.St.Med < best) {
				best = p.St.Med
			}
		}
		if best < 0 || best > o.maxMS {
			return 3
		}
	}
	return 0
}

func parseModes(s string) ([]string, error) {
	if s == "all" {
		return []string{"tcp", "https", "icmp"}, nil
	}
	var out []string
	for _, m := range splitList(s) {
		switch m {
		case "tcp", "https", "icmp":
			if !contains(out, m) {
				out = append(out, m)
			}
		default:
			return nil, fmt.Errorf("unknown mode %q (tcp, https, icmp, all)", m)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no mode given")
	}
	return out, nil
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// userAgent names the tool, its version, OS and CPU type, and whether it runs in CI. That is all the endpoint list
// request (the one counted) says about the machine.
func userAgent() string {
	ci := ""
	for _, k := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "BUILDKITE", "JENKINS_URL", "TF_BUILD"} {
		if os.Getenv(k) != "" {
			ci = "; ci"
			break
		}
	}
	return fmt.Sprintf("vps2cloud/%s (%s/%s%s; +https://v2c.1ms.dog)", version, runtime.GOOS, runtime.GOARCH, ci)
}

func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

func printList(w io.Writer, l *List, only []string) {
	ids := make([]string, 0, len(l.Vendors))
	for id := range l.Vendors {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		v := l.Vendors[id]
		if len(only) > 0 && !contains(only, id) && !contains(only, strings.ToLower(v.Name)) {
			continue
		}
		var regs []string
		for _, e := range l.Endpoints {
			if e.Vendor == id {
				regs = append(regs, e.Region)
			}
		}
		fmt.Fprintf(w, "%-13s %-18s %3d regions\n", id, v.Name, len(regs))
		if len(only) > 0 {
			for _, e := range l.Endpoints {
				if e.Vendor == id {
					fmt.Fprintf(w, "  %-22s %-24s %s\n", e.Region, e.City, e.Country)
				}
			}
		}
	}
	fmt.Fprintf(w, "\n%d regions · endpoints: %s (verified %s)\n", len(l.Endpoints), l.Source, l.Verified)
}
