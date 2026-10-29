package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// The menu runs when vps2cloud starts in a terminal with no options. On the first run it asks for a language and saves
// it. Then it asks what to test (or offers to change the language), the IP family, the measurement and the sample
// count (Enter keeps each default), and runs with the options it built, which it also prints so the same test can be
// run again without the menu.

var continentOrder = []string{"as", "eu", "na", "sa", "oc", "me", "af"}

type menu struct {
	in  *bufio.Reader
	out io.Writer
}

func (m menu) ask(prompt string, def int, max int) int {
	for {
		fmt.Fprintf(m.out, "%s [%d]: ", prompt, def)
		line, err := m.in.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			if err != nil {
				return -1 // input closed
			}
			return def
		}
		if n, e := strconv.Atoi(line); e == nil && n >= 1 && n <= max {
			return n
		}
		fmt.Fprintln(m.out, "  "+T("num", max))
	}
}

// pick reads a list like "1,3,5" or "2-4" of the numbered choices.
func (m menu) pick(prompt string, max int) []int {
	for {
		fmt.Fprintf(m.out, "%s: ", prompt)
		line, err := m.in.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" && err != nil {
			return nil
		}
		var out []int
		ok := line != ""
		for _, part := range strings.FieldsFunc(line, func(r rune) bool { return r == ',' || r == ' ' || r == '，' }) {
			a, b, isRange := strings.Cut(part, "-")
			lo, e1 := strconv.Atoi(a)
			hi := lo
			var e2 error
			if isRange {
				hi, e2 = strconv.Atoi(b)
			}
			if e1 != nil || e2 != nil || lo < 1 || hi > max || lo > hi {
				ok = false
				break
			}
			for i := lo; i <= hi; i++ {
				out = append(out, i)
			}
		}
		if ok && len(out) > 0 {
			return out
		}
		fmt.Fprintln(m.out, "  "+T("pickErr", max))
	}
}

func columns(w io.Writer, items []string) {
	width := 0
	for _, s := range items {
		if n := textWidth(s); n > width {
			width = n
		}
	}
	per := 3
	if width > 24 {
		per = 2
	}
	for i, s := range items {
		fmt.Fprint(w, "  "+padText(s, width+2, false))
		if (i+1)%per == 0 || i == len(items)-1 {
			fmt.Fprintln(w)
		}
	}
}

// textWidth counts terminal columns: CJK and other wide characters take two.
func textWidth(s string) int {
	n := 0
	for _, r := range stripANSI(s) {
		switch {
		case r == 0x200B || unicode.Is(unicode.Mn, r):
		case r >= 0x1100 && (r <= 0x115F || r == 0x2329 || r == 0x232A ||
			(r >= 0x2E80 && r <= 0xA4CF && r != 0x303F) || (r >= 0xAC00 && r <= 0xD7A3) ||
			(r >= 0xF900 && r <= 0xFAFF) || (r >= 0xFE30 && r <= 0xFE4F) || (r >= 0xFF00 && r <= 0xFF60) ||
			(r >= 0xFFE0 && r <= 0xFFE6) || (r >= 0x1F300 && r <= 0x1FAFF) || (r >= 0x20000 && r <= 0x3FFFD)):
			n += 2
		default:
			n++
		}
	}
	return n
}

func padText(s string, w int, right bool) string {
	n := textWidth(s)
	if n >= w {
		return s
	}
	if right {
		return strings.Repeat(" ", w-n) + s
	}
	return s + strings.Repeat(" ", w-n)
}

// hasRoute tells whether this machine has a route for an IP family. Connecting a UDP socket sends nothing.
func hasRoute(fam int) bool {
	network, addr := "udp4", "1.1.1.1:53"
	if fam == 6 {
		network, addr = "udp6", "[2606:4700:4700::1111]:53"
	}
	c, err := net.DialTimeout(network, addr, time.Second)
	if err != nil {
		return false
	}
	defer c.Close()
	ip := c.LocalAddr().(*net.UDPAddr).IP
	return fam == 4 || (ip.IsGlobalUnicast() && !ip.IsPrivate())
}

// langLabel names the language setting in the current language and in English, so it can be found again from a
// language one cannot read.
func langLabel() string {
	if lang == "en" {
		return "Language"
	}
	return T("langItem") + " / Language"
}

// chooseLang lists the languages (English first, Chinese second), sets the one picked and saves it.
func (m menu) chooseLang(first bool) bool {
	fmt.Fprintln(m.out)
	if first {
		fmt.Fprintln(m.out, "Language / 语言")
	} else {
		fmt.Fprintln(m.out, langLabel())
	}
	def := 1
	var items []string
	for i, l := range languages {
		items = append(items, fmt.Sprintf("%2d) %s", i+1, l.Name))
		if l.Code == lang {
			def = i + 1
		}
	}
	columns(m.out, items)
	n := m.ask(T("choice"), def, len(languages))
	if n < 0 {
		return false
	}
	lang = languages[n-1].Code
	c := loadConfig()
	c.Lang = lang
	if err := saveConfig(c); err == nil {
		fmt.Fprintln(m.out, T("langSaved", languages[n-1].Name))
	}
	return true
}

// wizard returns one set of options per run ("both" families is two runs), or nil when the input ends. askLang is
// set on the first run, before any language has been chosen.
func wizard(in io.Reader, out io.Writer, l *List, askLang bool) [][]string {
	m := menu{bufio.NewReader(in), out}
	if askLang && !m.chooseLang(true) {
		return nil
	}
	v6 := hasRoute(6)
	fams := "IPv4"
	if v6 {
		fams = "IPv4 + IPv6"
	}

	var args []string
	for done := false; !done; {
		fmt.Fprintf(out, "\n%s\n", T("title", version, len(l.Endpoints)))
		fmt.Fprintf(out, "%s\n\n", T("machine", fams))
		fmt.Fprintln(out, T("what"))
		fmt.Fprintf(out, "  1) %s\n", T("all", len(l.Endpoints)))
		fmt.Fprintf(out, "  2) %s\n", T("near"))
		fmt.Fprintf(out, "  3) %s\n", T("prov"))
		fmt.Fprintf(out, "  4) %s\n", T("cont"))
		fmt.Fprintf(out, "  5) %s\n", langLabel())
		done = true
		switch m.ask(T("choice"), 1, 5) {
		case -1:
			return nil
		case 2:
			args = append(args, "--near", "20")
		case 3:
			ids := make([]string, 0, len(l.Vendors))
			for id := range l.Vendors {
				ids = append(ids, id)
			}
			count := map[string]int{}
			for _, e := range l.Endpoints {
				count[e.Vendor]++
			}
			sort.Slice(ids, func(i, j int) bool {
				return count[ids[i]] > count[ids[j]] || count[ids[i]] == count[ids[j]] && ids[i] < ids[j]
			})
			var items []string
			for i, id := range ids {
				items = append(items, fmt.Sprintf("%2d) %s (%d)", i+1, l.Vendors[id].Name, count[id]))
			}
			fmt.Fprintln(out)
			columns(out, items)
			sel := m.pick(T("pickProv"), len(ids))
			if sel == nil {
				return nil
			}
			var names []string
			for _, i := range sel {
				names = append(names, ids[i-1])
			}
			args = append(args, "-p", strings.Join(names, ","))
		case 4:
			count := map[string]int{}
			for _, e := range l.Endpoints {
				count[e.Continent]++
			}
			var cs, items []string
			for _, c := range continentOrder {
				if count[c] > 0 {
					cs = append(cs, c)
					items = append(items, fmt.Sprintf("%d) %s (%d)", len(cs), continentName(c), count[c]))
				}
			}
			fmt.Fprintln(out)
			columns(out, items)
			sel := m.pick(T("pickCont"), len(cs))
			if sel == nil {
				return nil
			}
			var names []string
			for _, i := range sel {
				names = append(names, cs[i-1])
			}
			args = append(args, "--continent", strings.Join(names, ","))
		case 5:
			if !m.chooseLang(false) {
				return nil
			}
			done = false // back to the first question, now in the new language
		}
	}

	famRuns := [][]string{nil}
	if v6 {
		fmt.Fprintln(out, "\n"+T("ip"))
		fmt.Fprintln(out, "  1) "+T("v4"))
		fmt.Fprintln(out, "  2) "+T("v6"))
		fmt.Fprintln(out, "  3) "+T("both"))
		switch m.ask(T("choice"), 1, 3) {
		case -1:
			return nil
		case 2:
			famRuns = [][]string{{"-6"}}
		case 3:
			famRuns = [][]string{nil, {"-6"}}
		}
	}

	fmt.Fprintln(out, "\n"+T("how"))
	fmt.Fprintln(out, "  1) "+T("tcp"))
	fmt.Fprintln(out, "  2) "+T("https"))
	fmt.Fprintln(out, "  3) "+T("icmp"))
	fmt.Fprintln(out, "  4) "+T("allModes"))
	switch m.ask(T("choice"), 1, 4) {
	case -1:
		return nil
	case 2:
		args = append(args, "-m", "https")
	case 3:
		args = append(args, "-m", "icmp")
	case 4:
		args = append(args, "-m", "all")
	}

	fmt.Fprintln(out, "\n"+T("samples"))
	fmt.Fprintln(out, "  1) "+T("quick"))
	fmt.Fprintln(out, "  2) "+T("std"))
	fmt.Fprintln(out, "  3) "+T("precise"))
	switch m.ask(T("choice"), 2, 3) {
	case -1:
		return nil
	case 1:
		args = append(args, "-n", "5")
	case 3:
		args = append(args, "-n", "20")
	}
	fmt.Fprintln(out)

	var runs [][]string
	for _, f := range famRuns {
		runs = append(runs, append(append([]string(nil), args...), f...))
	}
	return runs
}

// commandLine shows options as a command to type, quoting only what needs it.
func commandLine(args []string) string {
	parts := []string{"vps2cloud"}
	for _, a := range args {
		if strings.ContainsAny(a, " *?'\"") {
			a = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		}
		parts = append(parts, a)
	}
	return strings.Join(parts, " ")
}
