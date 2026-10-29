package main

import (
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestWizard(t *testing.T) {
	l, err := parseList(embeddedList)
	if err != nil {
		t.Fatal(err)
	}
	v6 := hasRoute(6)
	cases := []struct {
		name, keys string
		want       [][]string
	}{
		{"all defaults", "\n\n\n\n", [][]string{{}}},
		{"nearest, https, quick", "2\n\n2\n1\n", [][]string{{"--near", "20", "-m", "https", "-n", "5"}}},
		{"two continents, icmp, precise", "4\n1,2\n\n3\n3\n", [][]string{{"--continent", "as,eu", "-m", "icmp", "-n", "20"}}},
		{"retry after bad input", "9\nx\n1\n\n4\n\n", [][]string{{"-m", "all"}}},
	}
	for _, c := range cases {
		keys := c.keys
		if !v6 {
			// without IPv6 the IP question is not asked: drop its answer (the second line after the scope answers)
			keys = dropIPAnswer(c.name, keys)
		}
		got := wizard(strings.NewReader(keys), io.Discard, l)
		for i := range got {
			if got[i] == nil {
				got[i] = []string{}
			}
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// dropIPAnswer removes the answer to the IP question from a key sequence written for a machine with IPv6.
func dropIPAnswer(name, keys string) string {
	lines := strings.SplitAfter(keys, "\n")
	ip := map[string]int{"all defaults": 1, "nearest, https, quick": 1, "two continents, icmp, precise": 2, "retry after bad input": 3}[name]
	return strings.Join(append(lines[:ip:ip], lines[ip+1:]...), "")
}

func TestWizardProviders(t *testing.T) {
	l, _ := parseList(embeddedList)
	keys := "3\n1,2\n\n\n\n"
	if !hasRoute(6) {
		keys = "3\n1,2\n\n\n"
	}
	got := wizard(strings.NewReader(keys), io.Discard, l)
	if len(got) != 1 || len(got[0]) != 2 || got[0][0] != "-p" || strings.Count(got[0][1], ",") != 1 {
		t.Fatalf("got %q", got)
	}
	// the two largest providers come first
	if p := got[0][1]; !strings.Contains(p, "oracle") || !strings.Contains(p, "gcp") {
		t.Errorf("expected the two largest providers, got %s", p)
	}
}

func TestWizardEOF(t *testing.T) {
	l, _ := parseList(embeddedList)
	if got := wizard(strings.NewReader(""), io.Discard, l); got != nil {
		t.Errorf("closed input should end the menu, got %q", got)
	}
}

func TestCommandLine(t *testing.T) {
	if got := commandLine([]string{"-r", "ap-*", "-6"}); got != "vps2cloud -r 'ap-*' -6" {
		t.Errorf("got %s", got)
	}
}
