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
// count. Enter keeps each default and 0 goes back a step (on the first question, 0 exits). It returns the options it
// built, which main runs and also prints so the same test can be run again without the menu.

var continentOrder = []string{"as", "eu", "na", "sa", "oc", "me", "af"}

type menu struct {
	in  *bufio.Reader
	out io.Writer
}

// What ask returns for 0 (back, or exit on the first question) and for the end of the input (Ctrl-D).
const back, end = 0, -1

func (m menu) line() (string, bool) {
	s, err := m.in.ReadString('\n')
	s = strings.TrimSpace(s)
	return s, !(s == "" && err != nil)
}

// question prints a title and numbered options, then a "0)" line (back, or exit on the first question).
func (m menu) question(title string, opts []string, first bool) {
	fmt.Fprintln(m.out, "\n"+heading(title))
	for i, o := range opts {
		fmt.Fprintf(m.out, "  %s) %s\n", key(i+1), o)
	}
	zero := T("back")
	if first {
		zero = T("exit")
	}
	fmt.Fprintf(m.out, "  %s) %s\n", key(0), paint(cDim, zero))
}

// ask reads a number from 0 to max; Enter gives def. It returns end when the input closes.
func (m menu) ask(def, max int) int {
	for {
		fmt.Fprintf(m.out, "%s %s: ", paint(cBold, T("choice")), paint(cDim, "["+strconv.Itoa(def)+"]"))
		s, ok := m.line()
		if !ok {
			return end
		}
		if s == "" {
			return def
		}
		if n, e := strconv.Atoi(s); e == nil && n >= 0 && n <= max {
			return n
		}
		fmt.Fprintln(m.out, "  "+paint(cYellow, T("num", max)))
	}
}

// pick reads a list like "1,3,5" or "2-4" of the numbered choices; "0" is back. It returns nil for back and
// end-of-input alike, with ok false only for the latter.
func (m menu) pick(prompt string, max int) (sel []int, ok bool) {
	for {
		fmt.Fprintf(m.out, "%s %s: ", paint(cBold, prompt), paint(cDim, "[0 = "+T("back")+"]"))
		s, ok := m.line()
		if !ok {
			return nil, false
		}
		if s == "0" {
			return nil, true
		}
		var out []int
		valid := s != ""
		for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '，' }) {
			a, b, isRange := strings.Cut(part, "-")
			lo, e1 := strconv.Atoi(a)
			hi := lo
			var e2 error
			if isRange {
				hi, e2 = strconv.Atoi(b)
			}
			if e1 != nil || e2 != nil || lo < 1 || hi > max || lo > hi {
				valid = false
				break
			}
			for i := lo; i <= hi; i++ {
				out = append(out, i)
			}
		}
		if valid && len(out) > 0 {
			return out, true
		}
		fmt.Fprintln(m.out, "  "+paint(cYellow, T("pickErr", max)))
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

// textWidth counts terminal columns: CJK and other wide characters take two; escape codes take none.
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

// chooseLang lists the languages (English first, Chinese second), sets the one picked and saves it. It returns back
// for 0 and end when the input closes.
func (m menu) chooseLang(first bool) int {
	title := langLabel()
	if first {
		title = "Language / 语言"
	}
	fmt.Fprintln(m.out, "\n"+heading(title))
	def := 1
	var items []string
	for i, l := range languages {
		items = append(items, fmt.Sprintf("%s) %s", key(i+1), l.Name))
		if l.Code == lang {
			def = i + 1
		}
	}
	columns(m.out, items)
	zero := T("back")
	if first {
		zero = T("exit")
	}
	fmt.Fprintf(m.out, "  %s) %s\n", key(0), paint(cDim, zero))
	n := m.ask(def, len(languages))
	if n <= 0 {
		return n
	}
	lang = languages[n-1].Code
	c := loadConfig()
	c.Lang = lang
	if err := saveConfig(c); err == nil {
		fmt.Fprintln(m.out, paint(cGreen, "✓ "+T("langSaved", languages[n-1].Name)))
	}
	return n
}

// wizard returns the options for the run, or nil when the user exits or the input ends. askLang is set on the first
// run, before any language has been chosen.
func wizard(in io.Reader, out io.Writer, l *List, askLang bool) []string {
	m := menu{bufio.NewReader(in), out}
	if askLang {
		if m.chooseLang(true) <= 0 {
			return nil
		}
	}
	v6 := hasRoute(6)
	fams := "IPv4"
	if v6 {
		fams = "IPv4 + IPv6"
	}

	// the answers, by step; a step that goes back clears its own answer
	var scope, ip, mode, samples []string
	const (
		stepScope = iota
		stepIP
		stepMode
		stepSamples
		stepDone
	)
	step := stepScope
	for step != stepDone {
		switch step {
		case stepScope:
			fmt.Fprintf(out, "\n%s\n%s\n", paint(cBold, T("title", version, len(l.Endpoints))), paint(cDim, T("machine", fams)))
			m.question(T("what"), []string{T("all", len(l.Endpoints)), T("near"), T("prov"), T("cont"), langLabel()}, true)
			switch m.ask(1, 5) {
			case end, back:
				return nil
			case 1:
				scope, step = nil, stepIP
			case 2:
				scope, step = []string{"--near", "20"}, stepIP
			case 3:
				if sel := m.providers(l); sel == nil {
					continue // back to this question (or the input ended: the next read returns end)
				} else {
					scope, step = sel, stepIP
				}
			case 4:
				if sel := m.continents(l); sel == nil {
					continue
				} else {
					scope, step = sel, stepIP
				}
			case 5:
				if m.chooseLang(false) == end {
					return nil
				}
			}
			if step == stepIP && !v6 {
				ip, step = nil, stepMode // no IPv6 here: nothing to ask
			}
		case stepIP:
			m.question(T("ip"), []string{T("v4"), T("v6"), T("both")}, false)
			switch m.ask(1, 3) {
			case end:
				return nil
			case back:
				step = stepScope
			case 1:
				ip, step = nil, stepMode
			case 2:
				ip, step = []string{"--ip", "6"}, stepMode
			case 3:
				ip, step = []string{"--ip", "both"}, stepMode
			}
		case stepMode:
			m.question(T("how"), []string{T("tcp"), T("https"), T("icmp"), T("allModes")}, false)
			switch m.ask(1, 4) {
			case end:
				return nil
			case back:
				step = stepIP
				if !v6 {
					step = stepScope
				}
			case 1:
				mode, step = nil, stepSamples
			case 2:
				mode, step = []string{"-m", "https"}, stepSamples
			case 3:
				mode, step = []string{"-m", "icmp"}, stepSamples
			case 4:
				mode, step = []string{"-m", "all"}, stepSamples
			}
		case stepSamples:
			m.question(T("samples"), []string{T("quick"), T("std"), T("precise")}, false)
			switch m.ask(2, 3) {
			case end:
				return nil
			case back:
				step = stepMode
			case 1:
				samples, step = []string{"-n", "5"}, stepDone
			case 2:
				samples, step = nil, stepDone
			case 3:
				samples, step = []string{"-n", "20"}, stepDone
			}
		}
	}
	fmt.Fprintln(out)
	args := []string{}
	for _, part := range [][]string{scope, ip, mode, samples} {
		args = append(args, part...)
	}
	return args
}

// providers lists the providers, largest first, and returns "-p id,id" or nil for back.
func (m menu) providers(l *List) []string {
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
		items = append(items, fmt.Sprintf("%s) %s %s", key(i+1), l.Vendors[id].Name, paint(cDim, "("+strconv.Itoa(count[id])+")")))
	}
	fmt.Fprintln(m.out, "\n"+heading(T("prov")))
	columns(m.out, items)
	sel, _ := m.pick(T("pickProv"), len(ids))
	if sel == nil {
		return nil
	}
	var names []string
	for _, i := range sel {
		names = append(names, ids[i-1])
	}
	return []string{"-p", strings.Join(names, ",")}
}

// continents lists the continents with regions and returns "--continent a,b" or nil for back.
func (m menu) continents(l *List) []string {
	count := map[string]int{}
	for _, e := range l.Endpoints {
		count[e.Continent]++
	}
	var cs, items []string
	for _, c := range continentOrder {
		if count[c] > 0 {
			cs = append(cs, c)
			items = append(items, fmt.Sprintf("%s) %s %s", key(len(cs)), continentName(c), paint(cDim, "("+strconv.Itoa(count[c])+")")))
		}
	}
	fmt.Fprintln(m.out, "\n"+heading(T("cont")))
	columns(m.out, items)
	sel, _ := m.pick(T("pickCont"), len(cs))
	if sel == nil {
		return nil
	}
	var names []string
	for _, i := range sel {
		names = append(names, cs[i-1])
	}
	return []string{"--continent", strings.Join(names, ",")}
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
