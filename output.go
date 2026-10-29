package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Tiers by median, the same bounds as Me2Cloud: T0 ≤ 30 ms, T1 ≤ 80, T2 ≤ 150, T3 ≤ 250, T4 above.
var grades = []float64{30, 80, 150, 250}

func tier(med float64) int {
	for i, g := range grades {
		if med <= g {
			return i
		}
	}
	return 4
}

// Use-case presets, as in Me2Cloud: max median ms, max jitter ms, max loss %; 0 = no limit.
var presets = map[string][3]float64{"game": {60, 10, 0}, "call": {150, 30, 1}, "ssh": {120, 20, 1}, "web": {250, 50, 2}}

func fits(st Stats, p [3]float64) bool {
	return st.OK > 0 && (p[0] == 0 || st.Med <= p[0]) && (p[1] == 0 || st.Jit <= p[1]) && st.Loss*100 <= p[2]
}

type View struct {
	Res    []*Result
	Modes  []string
	List   *List
	Origin *Origin
	ShowIP bool
	Fam, N int
	At     time.Time
	Took   time.Duration
	Preset string
	Top    int
	Link   string
	Color  bool
	SortBy string
	NoV6   int // regions left out under -6 for having no IPv6 address
}

// sortResults ranks by the first mode: answered hosts first, by the chosen key, then by median.
func sortResults(res []*Result, modes []string, by string) {
	key := func(st Stats) float64 {
		switch by {
		case "min":
			return st.Min
		case "p90":
			return st.P90
		case "jitter":
			return st.Jit
		case "loss":
			return st.Loss*1e6 + st.Med
		}
		return st.Med
	}
	sort.SliceStable(res, func(i, j int) bool {
		a, b := res[i].Primary(modes), res[j].Primary(modes)
		ao, bo := a != nil && a.St.OK > 0, b != nil && b.St.OK > 0
		if ao != bo {
			return ao
		}
		if !ao {
			return res[i].E.ID < res[j].E.ID
		}
		if by == "name" {
			return res[i].E.ID < res[j].E.ID
		}
		if ka, kb := key(a.St), key(b.St); ka != kb {
			return ka < kb
		}
		return a.St.Med < b.St.Med
	})
}

func (v View) headline() string {
	var m []string
	for _, x := range v.Modes {
		m = append(m, T("m_"+x))
	}
	return fmt.Sprintf("%s · %s · IPv%d · %s", strings.Join(m, " + "), T("nSamples", v.N), v.Fam, v.At.UTC().Format("2006-01-02 15:04 UTC"))
}

func (v View) from() string {
	o := v.Origin
	if o == nil {
		return T("unknownNet")
	}
	var parts []string
	if loc := strings.Trim(o.City+", "+o.Country, ", "); loc != "" {
		parts = append(parts, loc)
	}
	if o.Org != "" {
		parts = append(parts, fmt.Sprintf("%s (AS%d)", o.Org, o.ASN))
	}
	if v.ShowIP && o.IP != "" {
		parts = append(parts, o.IP)
	}
	return strings.Join(parts, " · ")
}

func f1(x float64) string {
	if x >= 100 {
		return strconv.FormatFloat(x, 'f', 0, 64)
	}
	return strconv.FormatFloat(x, 'f', 1, 64)
}

func (v View) vendorName(id string) string {
	if x, ok := v.List.Vendors[id]; ok && x.Name != "" {
		return x.Name
	}
	return id
}

func (v View) shown() []*Result {
	if v.Top > 0 && v.Top < len(v.Res) {
		return v.Res[:v.Top]
	}
	return v.Res
}

const (
	cReset = "\x1b[0m"
	cDim   = "\x1b[2m"
	cBold  = "\x1b[1m"
)

var tierColor = []string{"\x1b[32m", "\x1b[36m", "\x1b[33m", "\x1b[35m", "\x1b[31m"}

func (v View) c(code, s string) string {
	if !v.Color || s == "" {
		return s
	}
	return code + s + cReset
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// Table: one row per region, ranked; with several modes each gets a median column. Then the best region per provider.
func (v View) Table(w io.Writer) {
	fmt.Fprintf(w, "%s %s\n", v.c(cBold, "VPS2Cloud "+version), v.c(cDim, "· "+v.headline()))
	fmt.Fprintf(w, "%s %s\n\n", v.c(cDim, T("from")), v.from())
	head := []string{"#", T("h_provider"), T("h_region"), T("h_city"), T("h_median"), T("h_min"), T("h_p90"), T("h_jitter"), T("h_loss"), T("h_tier")}
	multi := len(v.Modes) > 1
	if multi {
		head = []string{"#", T("h_provider"), T("h_region"), T("h_city")}
		for _, m := range v.Modes {
			head = append(head, strings.ToUpper(m))
		}
		head = append(head, T("h_jitter"), T("h_loss"), T("h_tier"))
	}
	textTail := 1 // tier (and fit) are text at the end; the columns from the 5th up to them are numbers
	if v.Preset != "" {
		head = append(head, T("h_fit"))
		textTail = 2
	}
	var rows [][]string
	for i, r := range v.shown() {
		p := r.Primary(v.Modes)
		row := []string{strconv.Itoa(i + 1), v.vendorName(r.E.Vendor), r.E.Region, r.E.City}
		cell := func(m *ModeResult) string {
			if r.Err != "" {
				return v.c(cDim, r.Err)
			}
			if m == nil || m.St.OK == 0 {
				if m != nil && m.Err != "" {
					return v.c(cDim, m.Err)
				}
				return v.c(cDim, "–")
			}
			return f1(m.St.Med)
		}
		if multi {
			for _, m := range v.Modes {
				row = append(row, cell(r.Modes[m]))
			}
		} else {
			row = append(row, cell(p))
			if p != nil && p.St.OK > 0 {
				row = append(row, f1(p.St.Min), f1(p.St.P90))
			} else {
				row = append(row, "", "")
			}
		}
		if p != nil && p.St.OK > 0 {
			t := tier(p.St.Med)
			row = append(row, "±"+f1(p.St.Jit), fmt.Sprintf("%.0f%%", p.St.Loss*100), v.c(tierColor[t], "T"+strconv.Itoa(t)))
		} else {
			row = append(row, "", "", "")
		}
		if v.Preset != "" {
			if p != nil && fits(p.St, presets[v.Preset]) {
				row = append(row, v.c(tierColor[0], "✓"))
			} else {
				row = append(row, "")
			}
		}
		rows = append(rows, row)
	}
	widths := make([]int, len(head))
	for i, h := range head {
		widths[i] = textWidth(h)
	}
	for _, row := range rows {
		for i, cl := range row {
			if n := textWidth(cl); n > widths[i] {
				widths[i] = n
			}
		}
	}
	numeric := func(i int) bool { return i == 0 || (i >= 4 && i < len(head)-textTail) }
	line := func(cells []string, dim bool) {
		var parts []string
		for i, cl := range cells {
			parts = append(parts, padText(cl, widths[i], numeric(i)))
		}
		s := strings.TrimRight(strings.Join(parts, "  "), " ")
		if dim {
			s = v.c(cDim, s)
		}
		fmt.Fprintln(w, s)
	}
	line(head, true)
	for _, row := range rows {
		line(row, false)
	}
	if v.Top > 0 && v.Top < len(v.Res) {
		fmt.Fprintln(w, v.c(cDim, T("more", len(v.Res)-v.Top)))
	}
	// the best region of each provider, in rank order
	seen := map[string]bool{}
	var best []string
	for _, r := range v.Res {
		p := r.Primary(v.Modes)
		if seen[r.E.Vendor] || p == nil || p.St.OK == 0 {
			continue
		}
		seen[r.E.Vendor] = true
		best = append(best, fmt.Sprintf("%s %s %s ms", v.vendorName(r.E.Vendor), r.E.Region, f1(p.St.Med)))
	}
	if len(seen) > 1 {
		fmt.Fprintf(w, "\n%s\n", v.c(cBold, T("best")))
		for _, b := range best {
			fmt.Fprintln(w, "  "+b)
		}
	}
	ok := 0
	for _, r := range v.Res {
		if p := r.Primary(v.Modes); p != nil && p.St.OK > 0 {
			ok++
		}
	}
	noV6 := ""
	if v.NoV6 > 0 {
		noV6 = T("noV6", v.NoV6)
	}
	fmt.Fprintf(w, "\n%s\n", v.c(cDim, T("summary", ok, len(v.Res), noV6, v.Took.Round(time.Second), v.List.Source)))
	if v.Link != "" {
		fmt.Fprintf(w, "%s %s\n", v.c(cBold, T("link")), v.Link)
	}
}

// Markdown: a table to paste into a forum post or an issue, with the share link for the full result.
func (v View) Markdown(w io.Writer) {
	fmt.Fprintf(w, "**VPS2Cloud** %s %s · %s\n\n", T("from"), v.from(), v.headline())
	first := fmt.Sprintf("| # | %s | %s | %s |", T("h_provider"), T("h_region"), T("h_city"))
	cols := first + fmt.Sprintf(" %s | %s | %s |", T("h_median"), T("h_jitter"), T("h_loss"))
	if len(v.Modes) > 1 {
		cols = first
		for _, m := range v.Modes {
			cols += " " + strings.ToUpper(m) + " |"
		}
		cols += fmt.Sprintf(" %s | %s |", T("h_jitter"), T("h_loss"))
	}
	fmt.Fprintln(w, cols)
	sep := "|---:|---|---|---|"
	for i := 4; i < strings.Count(cols, "|")-1; i++ {
		sep += "---:|"
	}
	fmt.Fprintln(w, sep)
	for i, r := range v.shown() {
		p := r.Primary(v.Modes)
		if p == nil || p.St.OK == 0 {
			continue
		}
		row := fmt.Sprintf("| %d | %s | %s | %s |", i+1, v.vendorName(r.E.Vendor), r.E.Region, r.E.City)
		if len(v.Modes) > 1 {
			for _, m := range v.Modes {
				if x := r.Modes[m]; x != nil && x.St.OK > 0 {
					row += " " + f1(x.St.Med) + " |"
				} else {
					row += " – |"
				}
			}
		} else {
			row += " " + f1(p.St.Med) + " ms |"
		}
		row += fmt.Sprintf(" ±%s | %.0f%% |", f1(p.St.Jit), p.St.Loss*100)
		fmt.Fprintln(w, row)
	}
	if v.Top > 0 && v.Top < len(v.Res) {
		fmt.Fprintf(w, "\n_%s_", T("topOf", v.Top, len(v.Res)))
	}
	if v.Link != "" {
		fmt.Fprintf(w, " [%s](%s)", T("fullResult"), v.Link)
	}
	fmt.Fprintln(w)
}

type jsonStats struct {
	Samples int        `json:"samples"`
	OK      int        `json:"ok"`
	LossPct float64    `json:"loss_pct"`
	Min     *float64   `json:"min_ms"`
	P25     *float64   `json:"p25_ms"`
	Median  *float64   `json:"median_ms"`
	P75     *float64   `json:"p75_ms"`
	P90     *float64   `json:"p90_ms"`
	Max     *float64   `json:"max_ms"`
	Avg     *float64   `json:"avg_ms"`
	Jitter  *float64   `json:"jitter_ms"`
	Tier    *int       `json:"tier,omitempty"`
	Raw     []*float64 `json:"raw_ms"`
	Error   string     `json:"error,omitempty"`
	Reconn  int        `json:"new_connections,omitempty"`
}

func r3(x float64) *float64 { y := math.Round(x*1000) / 1000; return &y }

func toJSONStats(m *ModeResult) jsonStats {
	j := jsonStats{Samples: m.St.N, OK: m.St.OK, LossPct: math.Round(m.St.Loss * 100), Error: m.Err, Reconn: m.Reconn}
	for _, s := range m.Samples {
		if math.IsNaN(s) {
			j.Raw = append(j.Raw, nil)
		} else {
			j.Raw = append(j.Raw, r3(s))
		}
	}
	if m.St.OK > 0 {
		st := m.St
		t := tier(st.Med)
		j.Min, j.P25, j.Median, j.P75, j.P90, j.Max, j.Avg, j.Jitter, j.Tier = r3(st.Min), r3(st.P25), r3(st.Med), r3(st.P75), r3(st.P90), r3(st.Max), r3(st.Avg), r3(st.Jit), &t
	}
	return j
}

func (v View) JSON(w io.Writer) {
	type row struct {
		ID        string               `json:"id"`
		Provider  string               `json:"provider"`
		Region    string               `json:"region"`
		City      string               `json:"city"`
		Country   string               `json:"country"`
		Continent string               `json:"continent"`
		Host      string               `json:"host"`
		Addr      string               `json:"addr,omitempty"`
		Error     string               `json:"error,omitempty"`
		Fits      *bool                `json:"fits,omitempty"`
		Results   map[string]jsonStats `json:"results"`
	}
	o := v.Origin
	if o != nil && !v.ShowIP {
		c := *o
		c.IP = ""
		o = &c
	}
	out := struct {
		Tool      string    `json:"tool"`
		Version   string    `json:"version"`
		Time      time.Time `json:"time"`
		Modes     []string  `json:"modes"`
		Samples   int       `json:"samples"`
		Family    int       `json:"ip_family"`
		From      *Origin   `json:"from"`
		Endpoints string    `json:"endpoints"`
		Preset    string    `json:"preset,omitempty"`
		Link      string    `json:"link,omitempty"`
		Regions   []row     `json:"regions"`
	}{"vps2cloud", version, v.At.UTC(), v.Modes, v.N, v.Fam, o, v.List.Source, v.Preset, v.Link, nil}
	for _, r := range v.shown() {
		x := row{ID: r.E.ID, Provider: r.E.Vendor, Region: r.E.Region, City: r.E.City, Country: r.E.Country, Continent: r.E.Continent, Host: r.E.Host(v.Fam), Addr: r.Addr, Error: r.Err, Results: map[string]jsonStats{}}
		for m, mr := range r.Modes {
			x.Results[m] = toJSONStats(mr)
		}
		if v.Preset != "" {
			f := false
			if p := r.Primary(v.Modes); p != nil {
				f = fits(p.St, presets[v.Preset])
			}
			x.Fits = &f
		}
		out.Regions = append(out.Regions, x)
	}
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	e.Encode(out)
}

func (v View) CSV(w io.Writer) {
	cw := csv.NewWriter(w)
	cw.Write([]string{"id", "provider", "region", "city", "country", "mode", "samples", "ok", "loss_pct", "min_ms", "median_ms", "p90_ms", "max_ms", "jitter_ms", "tier", "error"})
	num := func(x float64, ok bool) string {
		if !ok {
			return ""
		}
		return strconv.FormatFloat(math.Round(x*1000)/1000, 'f', -1, 64)
	}
	for _, r := range v.shown() {
		for _, m := range v.Modes {
			mr := r.Modes[m]
			if mr == nil {
				cw.Write([]string{r.E.ID, r.E.Vendor, r.E.Region, r.E.City, r.E.Country, m, "", "", "", "", "", "", "", "", "", r.Err})
				continue
			}
			st, ok := mr.St, mr.St.OK > 0
			t := ""
			if ok {
				t = strconv.Itoa(tier(st.Med))
			}
			cw.Write([]string{r.E.ID, r.E.Vendor, r.E.Region, r.E.City, r.E.Country, m, strconv.Itoa(st.N), strconv.Itoa(st.OK), num(st.Loss*100, true),
				num(st.Min, ok), num(st.Med, ok), num(st.P90, ok), num(st.Max, ok), num(st.Jit, ok), t, mr.Err})
		}
	}
	cw.Flush()
}
