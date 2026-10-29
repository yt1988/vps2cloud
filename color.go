package main

import "strings"

// Colors for a terminal. useColor is on when the output is a terminal, unless NO_COLOR is set or --no-color is given;
// pipes, files and JSON/CSV never get escape codes. Tiers use the same green → red order as Me2Cloud.
var useColor bool

const (
	cReset  = "\x1b[0m"
	cBold   = "\x1b[1m"
	cDim    = "\x1b[2m"
	cRed    = "\x1b[31m"
	cGreen  = "\x1b[32m"
	cYellow = "\x1b[33m"
	cBlue   = "\x1b[34m"
	cCyan   = "\x1b[36m"
)

// tierColor: T0 green, T1 cyan, T2 yellow, T3 magenta, T4 red.
var tierColor = []string{"\x1b[32m", "\x1b[36m", "\x1b[33m", "\x1b[35m", "\x1b[31m"}

func paint(code, s string) string {
	if !useColor || s == "" {
		return s
	}
	return code + s + cReset
}

// heading prints a section or question title: bold, with a cyan marker.
func heading(s string) string { return paint(cCyan, "›") + " " + paint(cBold, s) }

// key shows a menu number.
func key(n int) string { return paint(cCyan, strings.TrimSpace(itoa(n))) }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
