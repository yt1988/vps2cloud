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
)

// The menu runs when vps2cloud starts in a terminal with no options. It asks what to test, the IP family, the
// measurement and the sample count (Enter keeps each default), then runs with the options it built, which it also
// prints so the same test can be run again without the menu.

var continentNames = map[string]string{"as": "Asia", "eu": "Europe", "na": "North America", "sa": "South America", "oc": "Oceania", "me": "Middle East", "af": "Africa"}
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
		fmt.Fprintf(m.out, "  Enter a number from 1 to %d.\n", max)
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
		for _, part := range strings.FieldsFunc(line, func(r rune) bool { return r == ',' || r == ' ' }) {
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
		fmt.Fprintf(m.out, "  Enter one or more numbers from 1 to %d, like 1,3 or 2-5.\n", max)
	}
}

func columns(w io.Writer, items []string) {
	width := 0
	for _, s := range items {
		if n := len([]rune(s)); n > width {
			width = n
		}
	}
	per := 3
	if width > 24 {
		per = 2
	}
	for i, s := range items {
		fmt.Fprintf(w, "  %-*s", width+2, s)
		if (i+1)%per == 0 || i == len(items)-1 {
			fmt.Fprintln(w)
		}
	}
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

// wizard returns one set of options per run ("both" families is two runs), or nil when the input ends.
func wizard(in io.Reader, out io.Writer, l *List) [][]string {
	m := menu{bufio.NewReader(in), out}
	fmt.Fprintf(out, "\nVPS2Cloud %s · latency from this machine to %d cloud regions\n", version, len(l.Endpoints))
	v6 := hasRoute(6)
	fams := "IPv4"
	if v6 {
		fams = "IPv4 + IPv6"
	}
	fmt.Fprintf(out, "This machine: %s · Enter keeps the [default] · Ctrl-C quits\n\n", fams)

	var args []string
	fmt.Fprintln(out, "What to test?")
	fmt.Fprintf(out, "  1) All %d regions\n", len(l.Endpoints))
	fmt.Fprintln(out, "  2) The 20 nearest regions (a quick scan finds them)")
	fmt.Fprintln(out, "  3) Choose providers")
	fmt.Fprintln(out, "  4) Choose continents")
	switch m.ask("Choice", 1, 4) {
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
		sel := m.pick("Providers (e.g. 1,3 or 2-5)", len(ids))
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
				items = append(items, fmt.Sprintf("%d) %s (%d)", len(cs), continentNames[c], count[c]))
			}
		}
		fmt.Fprintln(out)
		columns(out, items)
		sel := m.pick("Continents (e.g. 1,2)", len(cs))
		if sel == nil {
			return nil
		}
		var names []string
		for _, i := range sel {
			names = append(names, cs[i-1])
		}
		args = append(args, "--continent", strings.Join(names, ","))
	}

	famRuns := [][]string{nil}
	if v6 {
		fmt.Fprintln(out, "\nIP version?")
		fmt.Fprintln(out, "  1) IPv4")
		fmt.Fprintln(out, "  2) IPv6 (regions without IPv6 are skipped)")
		fmt.Fprintln(out, "  3) Both, one after the other")
		switch m.ask("Choice", 1, 3) {
		case -1:
			return nil
		case 2:
			famRuns = [][]string{{"-6"}}
		case 3:
			famRuns = [][]string{nil, {"-6"}}
		}
	}

	fmt.Fprintln(out, "\nHow to measure?")
	fmt.Fprintln(out, "  1) TCP handshake: the network round trip alone")
	fmt.Fprintln(out, "  2) HTTPS: a request on an open connection, as an app sees it (slower to run)")
	fmt.Fprintln(out, "  3) ICMP ping (needs root)")
	fmt.Fprintln(out, "  4) All three side by side")
	switch m.ask("Choice", 1, 4) {
	case -1:
		return nil
	case 2:
		args = append(args, "-m", "https")
	case 3:
		args = append(args, "-m", "icmp")
	case 4:
		args = append(args, "-m", "all")
	}

	fmt.Fprintln(out, "\nSamples per region?")
	fmt.Fprintln(out, "  1) Quick, 5")
	fmt.Fprintln(out, "  2) Standard, 10")
	fmt.Fprintln(out, "  3) Precise, 20")
	switch m.ask("Choice", 2, 3) {
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
