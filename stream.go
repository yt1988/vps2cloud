package main

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// streamer formats one line per region as it finishes, in finishing order, to stderr: "[ 12/392]  AWS  eu-central-1
// Frankfurt  4.8 T0". The sorted table still comes once at the end, on stdout.
type streamer struct {
	list            *List
	modes           []string
	fams            bool // a run over both IP families tags each line with its family
	color           bool
	wV, wR, wC, wNo int
}

func newStreamer(list *List, eps []Endpoint, modes []string, fams []int, color bool) *streamer {
	s := &streamer{list: list, modes: modes, fams: len(fams) > 1, color: color}
	for _, e := range eps {
		s.wV = max(s.wV, utf8.RuneCountInString(s.vendor(e.Vendor)))
		s.wR = max(s.wR, utf8.RuneCountInString(e.Region))
		s.wC = max(s.wC, utf8.RuneCountInString(e.City))
	}
	s.wNo = len(strconv.Itoa(len(eps) * len(fams)))
	return s
}

func (s *streamer) vendor(id string) string {
	if x, ok := s.list.Vendors[id]; ok && x.Name != "" {
		return x.Name
	}
	return id
}

func (s *streamer) paint(code, t string) string {
	if !s.color || t == "" {
		return t
	}
	return code + t + cReset
}

func padR(t string, w int) string {
	if n := utf8.RuneCountInString(t); n < w {
		return t + strings.Repeat(" ", w-n)
	}
	return t
}

func padL(t string, w int) string {
	if n := utf8.RuneCountInString(t); n < w {
		return strings.Repeat(" ", w-n) + t
	}
	return t
}

// line is the region's median in each mode asked for and the tier of the first, or why it has none.
func (s *streamer) line(done, total int, j Job, r *Result) string {
	no := "[" + padL(strconv.Itoa(done), s.wNo) + "/" + strconv.Itoa(total) + "]"
	b := []string{s.paint(cDim, no), padR(s.vendor(j.E.Vendor), s.wV), padR(j.E.Region, s.wR), s.paint(cDim, padR(j.E.City, s.wC))}
	if s.fams {
		b = append(b, s.paint(cDim, "IPv"+strconv.Itoa(j.Fam)))
	}
	if r.Err != "" {
		return strings.Join(append(b, s.paint(cDim, r.Err)), "  ")
	}
	for _, m := range s.modes {
		mr := r.Modes[m]
		var cell string
		switch {
		case mr != nil && mr.St.OK > 0:
			cell = s.paint(tierColor[tier(mr.St.Med)], padL(f1(mr.St.Med), 6))
		case mr != nil && mr.Err != "":
			cell = s.paint(cDim, padL(mr.Err, 6))
		default:
			cell = s.paint(cDim, padL("–", 6))
		}
		if len(s.modes) > 1 {
			cell = s.paint(cDim, strings.ToUpper(m)) + " " + cell
		}
		b = append(b, cell)
	}
	if p := r.Primary(s.modes); p != nil && p.St.OK > 0 {
		t := tier(p.St.Med)
		b = append(b, s.paint(tierColor[t], "T"+strconv.Itoa(t)))
	}
	return strings.Join(b, "  ")
}
