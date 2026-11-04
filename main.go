// VPS2Cloud measures latency, jitter and loss from the machine it runs on to the regions of 22 cloud providers.
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
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

var version = "dev" // set at build time

// Hosts of one provider measured at once. Each is a separate regional server and still gets one sample at a time with
// a pause in between, so four at once is as gentle per host as Me2Cloud (two) while a full run takes half as long.
const vendorCap = 4

// usageText is the help: the title, the group headings and the exit codes in the user's language, the options in
// English. The counts come from the built-in endpoint list.
func usageText() string {
	n, v := 0, 0
	if l, err := parseList(embeddedList); err == nil {
		n, v = len(l.Endpoints), len(l.Vendors)
	}
	return fmt.Sprintf(usageFmt, T("u_title", n, v), T("u_what"), T("u_how"), vendorCap, T("u_out"), T("u_list"), T("u_exit"))
}

const usageFmt = `%s

Usage: vps2cloud [options]

%s
  -p, --provider LIST     providers, comma separated (aws,gcp,hetzner …; --list shows all)
  -r, --region LIST       regions by code, id or city, wildcards allowed ("ap-*", "*tokyo*")
      --continent LIST    as, eu, na, sa, oc, me, af
      --cdn               test the nearest edge of each CDN (Cloudflare, Akamai, CloudFront, Google, Fastly,
                          Azure Front Door) instead of cloud regions, by HTTPS; -p picks some (-p cloudflare,fastly)
      --country LIST      ISO codes of the region's country (jp,de,us …)

%s
  -m, --mode MODE         tcp (default), https, icmp, or all (comma lists work too: tcp,icmp)
  -n, --count N           samples per region (default 10)
  -4 / -6                 IPv4 (default) or IPv6
      --ip both           IPv4 and IPv6 in one run, reported as two parts of one result
  -t, --timeout DUR       per sample (default 2s)
      --gap DUR           pause between samples to one host (default 250ms)
  -C, --concurrency N     hosts measured at once (default 24; at most %d per provider)

%s
  -o, --output FORMAT     table (default), or md, json, csv: saved to a file in the current folder
                          (vps2cloud-<date>-<time>.<ext>) while the table shows on screen
      --file NAME         the file to save to instead
      --stdout            print the md, json or csv to stdout instead of a file (for pipes)
      --sort KEY          median (default), min, p90, jitter, loss, name
      --top N             only the N best regions (md defaults to 20)
      --preset NAME       mark regions that fit: game, call, ssh, web
      --long-link         print the full share link instead of a short s.1ms.dog one (nothing is uploaded)
      --no-link           don't print a share link
      --show-ip           include this machine's public IP (never in share links)
      --no-color          plain text (also with NO_COLOR set)
  -q, --quiet             no progress or per-region lines (on stderr)
      --max-ms N          exit with status 3 when the best median is above N ms

%s
      --offline           use the list built into this binary, no download
      --endpoints FILE    use a local list (Me2Cloud's endpoints.json format)
      --list              show providers and regions, then exit
      --lang CODE         language of the output: en, zh-CN, zh-TW, ja, ko, es, fr, de, pt, ru
                          (the menu asks once and remembers it; V2C_LANG works too)
  -v, --version
      --update            update to the latest release now

Each run fetches the latest endpoint list from m2c.1ms.dog. That request is counted (tool version, OS and CPU type,
whether it runs in CI); --offline skips it. Besides the measurement, the only other requests are Cloudflare's /meta
for the "from" line and, at the end, s.1ms.dog, which keeps the full share link (and so the result in it) to give a
short one. --long-link prints the full link instead and uploads nothing.

At most once an hour it also asks GitHub for the latest release and, in a terminal, offers to update (the new binary
is checked against the release checksums). --offline or V2C_NO_UPDATE=1 turns that off.

%s
`

type opts struct {
	providers, regions, continents, countries             string
	mode, output, sortBy, preset, endpoints, timeout, gap string
	n, conc, top                                          int
	v4, v6, noLink, showIP, noColor, quiet, offline, list bool
	longLink, toStdout, cdn                               bool
	file                                                  string
	maxMS                                                 float64
	ver                                                   bool
	lang                                                  string
	ip                                                    string
}

func main() {
	// net/http logs HTTP/2 frames it does not handle (ORIGIN frames from Cloudflare and Google); none matter here
	log.SetOutput(io.Discard)
	// old Windows consoles show color codes as text until asked to read them; where that fails, no colors
	if !enableVT() {
		os.Setenv("NO_COLOR", "1")
	}
	// language: --lang (read in run), then V2C_LANG, then the saved choice, then English
	askLang := true
	if l := knownLang(os.Getenv("V2C_LANG")); l != "" {
		lang, askLang = l, false
	} else if l := knownLang(loadConfig().Lang); l != "" {
		lang, askLang = l, false
	}
	args := os.Args[1:]
	if code, stop := checkUpdate(args, userAgent()); stop {
		os.Exit(code)
	}
	if len(args) > 0 || !isTTY(os.Stdin) || !isTTY(os.Stdout) {
		code := run(args, os.Stdout, os.Stderr)
		if len(args) > 0 && isTTY(os.Stdin) && isTTY(os.Stdout) && !justLooking(args) {
			rememberRun(args, code)
		}
		os.Exit(code)
	}
	// no options in a terminal: the menu, then the run it set up
	l, err := loadList("", false, userAgent())
	if err != nil {
		os.Exit(run(args, os.Stdout, os.Stderr))
	}
	preloaded = l
	useColor = os.Getenv("NO_COLOR") == ""
	chosen := wizard(os.Stdin, os.Stdout, l, askLang, loadConfig().Last)
	if chosen == nil {
		os.Exit(0) // 0 on the first question, or the input ended
	}
	code := run(chosen, os.Stdout, os.Stderr)
	fmt.Fprintf(os.Stdout, "\n%s\n  %s\n", paint(cDim, T("again")), paint(cCyan, commandLine(chosen)))
	rememberRun(chosen, code)
	os.Exit(code)
}

// rememberRun keeps the options of a finished run (also one above --max-ms), so the menu can offer the same test
// again. Runs cut short, or that failed, are not worth repeating.
func rememberRun(args []string, code int) {
	if code != 0 && code != 3 {
		return
	}
	c := loadConfig()
	c.Last = args
	saveConfig(c)
}

// preloaded is the endpoint list the menu already fetched, so a run doesn't fetch (and count) it again.
var preloaded *List

func run(args []string, stdout, stderr io.Writer) int {
	var o opts
	fs := flag.NewFlagSet("vps2cloud", flag.ContinueOnError)
	// the flag package would print its own message and the whole help; a wrong option gets one line instead
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
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
	str(&o.lang, "", "lang")
	str(&o.ip, "", "ip")
	str(&o.timeout, "2s", "t", "timeout")
	str(&o.gap, "250ms", "gap")
	num(&o.n, 10, "n", "count")
	num(&o.conc, 24, "C", "concurrency")
	num(&o.top, -1, "top")
	flg(&o.v4, "4")
	flg(&o.v6, "6")
	flg(&o.noLink, "no-link")
	flg(&o.longLink, "long-link")
	flg(&o.toStdout, "stdout")
	str(&o.file, "", "file")
	flg(&o.showIP, "show-ip")
	flg(&o.noColor, "no-color")
	flg(&o.quiet, "q", "quiet")
	flg(&o.offline, "offline")
	flg(&o.cdn, "cdn")
	flg(&o.list, "list")
	flg(&o.ver, "v", "version")
	fs.Float64Var(&o.maxMS, "max-ms", 0, "")
	// --lang first, so the help and any complaint about the other options come in that language
	if l := knownLang(langArg(args)); l != "" {
		lang = l
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, usageText())
			return 0
		}
		fmt.Fprintln(stderr, "vps2cloud: "+flagError(fs, err))
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "vps2cloud: unexpected argument %q · %s\n", fs.Arg(0), T("seeHelp"))
		return 2
	}
	if o.lang != "" && knownLang(o.lang) == "" {
		fmt.Fprintf(stderr, "vps2cloud: unknown language %q (en, zh-CN, zh-TW, ja, ko, es, fr, de, pt, ru)\n", o.lang)
		return 2
	}
	if o.ver {
		fmt.Fprintf(stdout, "vps2cloud %s %s/%s\n", version, runtime.GOOS, runtime.GOARCH)
		return 0
	}
	fail := func(format string, a ...any) int { fmt.Fprintf(stderr, "vps2cloud: "+format+"\n", a...); return 2 }

	// CDNs are measured by HTTPS unless -m says otherwise: the edge's answer, on a warm connection
	if o.cdn {
		mset := false
		fs.Visit(func(f *flag.Flag) { mset = mset || f.Name == "m" || f.Name == "mode" })
		if !mset {
			o.mode = "https"
		}
	}
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
	fams := []int{4}
	switch {
	case o.v4 && o.v6:
		fams = []int{4, 6}
	case o.v6:
		fams = []int{6}
	}
	switch strings.ToLower(o.ip) {
	case "":
	case "4":
		fams = []int{4}
	case "6":
		fams = []int{6}
	case "both", "46", "4,6", "dual":
		fams = []int{4, 6}
	default:
		return fail("unknown --ip %q (4, 6, both)", o.ip)
	}

	ua := userAgent()
	list, err := preloaded, error(nil)
	if list == nil || o.endpoints != "" || o.offline {
		list, err = loadList(o.endpoints, o.offline, ua)
	}
	if err != nil {
		fmt.Fprintf(stderr, "vps2cloud: endpoint list: %v\n", err)
		return 1
	}
	if o.list {
		printList(stdout, list, splitList(o.providers))
		return 0
	}
	eps, err := list.Select(Filter{splitList(o.providers), splitList(o.regions), splitList(o.continents), splitList(o.countries)})
	if o.cdn {
		eps, err = list.SelectCDNs(splitList(o.providers)), nil
		if len(list.CDNs) == 0 {
			return fail("%s", T("noCDN"))
		}
	}
	if err != nil {
		return fail("%v", err)
	}
	if len(eps) == 0 {
		return fail("%s", T("noMatch"))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	useColor = !o.noColor && os.Getenv("NO_COLOR") == "" && isTTY(stdout)
	showProgress := !o.quiet && isTTY(stderr)
	pl := Plan{Probe: Probe{N: o.n, Gap: gap, Timeout: timeout, UA: ua}, Modes: modes, Fams: fams, Concurrency: o.conc, VendorCap: vendorCap, ICMPOK: map[int]bool{}}

	if contains(modes, "icmp") {
		var icmpErr error
		for _, f := range fams {
			if c, _, err := icmpListen(f); err == nil {
				c.Close()
				pl.ICMPOK[f] = true
			} else {
				icmpErr = err
			}
		}
		if icmpErr != nil && len(pl.ICMPOK) == 0 && len(modes) == 1 {
			return fail("ICMP needs root, CAP_NET_RAW (setcap cap_net_raw+ep vps2cloud) or a ping_group_range that covers you: %v", icmpErr)
		} else if icmpErr != nil && !o.quiet {
			fmt.Fprintln(stderr, paint(cYellow, "vps2cloud: "+T("icmpSkip")))
		}
	}

	start := time.Now()
	orig := make(chan *Origin, 1)
	go func() { orig <- origin(fams[0], ua) }()
	progressColor := !o.noColor && os.Getenv("NO_COLOR") == "" && isTTY(stderr)
	// in a color terminal wide enough, a running Shiba; otherwise one line of text
	var dog *runner
	if showProgress && progressColor {
		if f, ok := stderr.(*os.File); ok {
			dog = startRunner(stderr, termCols(f))
		}
	}
	// In a terminal the progress is one line (the count, the region that just finished, the best so far), replaced in
	// place; the sorted table follows once. Piped or in CI, each region's line is printed as it finishes.
	st := newStreamer(list, eps, modes, fams, progressColor)
	var best *Result
	progress := func(phase string) func(done, total int, j Job, r *Result) {
		if o.quiet {
			return nil
		}
		return func(done, total int, j Job, r *Result) {
			if !showProgress {
				if r != nil {
					fmt.Fprintln(stderr, st.line(done, total, j, r))
				}
				return
			}
			if r != nil {
				if p := r.Primary(modes); p != nil && p.St.OK > 0 && (best == nil || p.St.Med < best.Primary(modes).St.Med) {
					best = r
				}
			}
			fam := ""
			if len(fams) > 1 {
				fam = " IPv" + strconv.Itoa(j.Fam)
			}
			lead := ""
			if best != nil {
				lead = "  ·  " + T("bestSoFar") + ": " + strings.TrimSpace(list.VendorName(best.E.Vendor)+" "+regionText(best.E)) + " " + f1(best.Primary(modes).St.Med) + " ms"
			}
			if dog != nil {
				dog.update(done, total, fmt.Sprintf("%s  %s%s%s", cBold+strconv.Itoa(done)+"/"+strconv.Itoa(total)+cReset, j.E.ID, cDim+fam+cReset, cDim+lead+cReset))
				return
			}
			line := fmt.Sprintf("%s %d/%d  %s%s%s", phase, done, total, j.E.ID, fam, lead)
			if progressColor {
				line = fmt.Sprintf("%s %s  %s%s%s", cCyan+phase+cReset, cBold+strconv.Itoa(done)+"/"+strconv.Itoa(total)+cReset, j.E.ID, cDim+fam+cReset, cDim+lead+cReset)
			}
			fmt.Fprint(stderr, "\r\x1b[K"+line)
		}
	}

	res := pl.Run(ctx, eps, progress(T("measuring")))
	if dog != nil {
		dog.finish()
	} else if showProgress {
		fmt.Fprint(stderr, "\r\x1b[K")
	}
	if !o.quiet && !showProgress {
		fmt.Fprintln(stderr) // the streamed lines end; the sorted table follows
	}
	// interrupted: show the regions measured so far, then exit with 130 as an interrupted program does
	interrupted := ctx.Err() != nil
	if interrupted {
		kept := res[:0]
		for _, r := range res {
			if r != nil {
				kept = append(kept, r)
			}
		}
		if len(kept) == 0 {
			fmt.Fprintln(stderr, "vps2cloud: interrupted")
			return 130
		}
		fmt.Fprintln(stderr, paint(cYellow, "vps2cloud: "+TN("partial", len(kept), len(kept), len(res))))
		res = kept
	}
	// one section per family; regions without an IPv6 address are left out of the IPv6 one and counted instead
	var secs []Section
	for _, f := range fams {
		s := Section{Fam: f}
		for _, r := range res {
			if r.Fam != f {
				continue
			}
			if r.Err == errNoV6 {
				s.NoV6++
				continue
			}
			s.Res = append(s.Res, r)
		}
		sortResults(s.Res, modes, o.sortBy)
		secs = append(secs, s)
	}

	proxy, proxyDetail := proxySigns(res, modes)
	v := View{Secs: secs, Modes: modes, List: list, Origin: <-orig, ShowIP: o.showIP, N: o.n, At: start, Took: time.Since(start),
		Preset: o.preset, Top: o.top, SortBy: o.sortBy, Proxy: proxy, ProxyDetail: proxyDetail}
	if v.Top < 0 {
		v.Top = 0
		if o.output == "md" {
			v.Top = 20
		}
	}
	if !o.noLink {
		// the short link stores the full one on s.1ms.dog; without it (--long-link, --offline, or the service not
		// answering) the full link carries the result by itself
		v.LinkFull = shareLink(secs, list, modes, o.n, v.Origin, start, proxy != "")
		v.Link = v.LinkFull
		if !o.longLink && !o.offline {
			if s, err := shorten(v.LinkFull, ua); err == nil {
				v.Link = s
			} else if !o.quiet {
				fmt.Fprintln(stderr, paint(cDim, "vps2cloud: "+T("shortFail")))
			}
		}
	}
	// Markdown, JSON and CSV go to a file next to where the tool runs (or to stdout with --stdout, for pipes); the
	// table always shows in the terminal
	if o.output == "table" || !o.toStdout {
		t := v
		if t.Top < 0 || o.output == "md" && o.top < 0 {
			t.Top = 0
		}
		t.Table(stdout)
	}
	if o.output != "table" {
		write := map[string]func(io.Writer){"md": v.Markdown, "json": v.JSON, "csv": v.CSV}[o.output]
		if o.toStdout {
			write(stdout)
		} else {
			name := o.file
			if name == "" {
				name = fmt.Sprintf("vps2cloud-%s.%s", start.Format("20060102-150405"), o.output)
			}
			f, err := os.Create(name)
			if err != nil {
				fmt.Fprintf(stderr, "vps2cloud: %v\n", err)
				return 1
			}
			write(f)
			f.Close()
			if abs, err := filepath.Abs(name); err == nil {
				name = abs
			}
			fmt.Fprintf(stdout, "%s %s\n", paint(cBold, T("saved")), paint(cGreen, name))
		}
	}
	if interrupted {
		return 130
	}
	if o.maxMS > 0 {
		best := -1.0
		for _, s := range secs {
			for _, r := range s.Res {
				if p := r.Primary(modes); p != nil && p.St.OK > 0 && (best < 0 || p.St.Med < best) {
					best = p.St.Med
				}
			}
		}
		if best < 0 || best > o.maxMS {
			return 3
		}
	}
	return 0
}

// langArg finds --lang in the raw options before they are parsed, so even a parse error can speak that language.
func langArg(args []string) string {
	for i, a := range args {
		name, val, eq := strings.Cut(strings.TrimLeft(a, "-"), "=")
		if name != "lang" || !strings.HasPrefix(a, "-") {
			continue
		}
		if eq {
			return val
		}
		if i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// flagError turns the flag package's message into one line in the user's language: an unknown option gets the
// nearest real one as a suggestion (a prefix such as --cont, or a spelling one or two letters off).
func flagError(fs *flag.FlagSet, err error) string {
	msg := err.Error()
	name, ok := strings.CutPrefix(msg, "flag provided but not defined: -")
	if !ok {
		return msg + " · " + T("seeHelp")
	}
	name = strings.TrimLeft(name, "-")
	var names []string
	fs.VisitAll(func(f *flag.Flag) { names = append(names, f.Name) })
	out := T("badFlag", name)
	if s := nearestFlag(name, names); s != "" {
		out += " · " + T("didYouMean", s)
	}
	return out + " · " + T("seeHelp")
}

func nearestFlag(name string, names []string) string {
	best, bestD := "", 3
	for _, n := range names {
		if len(n) < 2 {
			continue // the one-letter aliases are never what a typo meant
		}
		d := editDistance(name, n)
		if len(name) >= 3 && strings.HasPrefix(n, name) {
			d = 0
		}
		if d < bestD || d == bestD && len(n) > len(best) {
			best, bestD = n, d
		}
	}
	return best
}

func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
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
	return fmt.Sprintf("vps2cloud/%s (%s/%s%s; +https://m2c.1ms.dog)", version, runtime.GOOS, runtime.GOARCH, ci)
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
