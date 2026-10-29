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
		got := wizard(strings.NewReader(keys), io.Discard, l, false)
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
	got := wizard(strings.NewReader(keys), io.Discard, l, false)
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
	if got := wizard(strings.NewReader(""), io.Discard, l, false); got != nil {
		t.Errorf("closed input should end the menu, got %q", got)
	}
}

func TestCommandLine(t *testing.T) {
	if got := commandLine([]string{"-r", "ap-*", "-6"}); got != "vps2cloud -r 'ap-*' -6" {
		t.Errorf("got %s", got)
	}
}

// isolate the saved language from the real config directory
func tempConfig(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", d)
	t.Setenv("APPDATA", d)
	t.Setenv("HOME", d)
}

func TestWizardLanguage(t *testing.T) {
	tempConfig(t)
	defer func() { lang = "en" }()
	l, _ := parseList(embeddedList)
	ip := "\n"
	if !hasRoute(6) {
		ip = ""
	}
	// first run: pick 简体中文 (2), then the defaults; the choice is saved
	lang = "en"
	var out strings.Builder
	got := wizard(strings.NewReader("2\n\n"+ip+"\n\n"), &out, l, true)
	if len(got) != 1 || lang != "zh-CN" || loadConfig().Lang != "zh-CN" {
		t.Fatalf("first run: runs %q, lang %s, saved %q", got, lang, loadConfig().Lang)
	}
	if !strings.Contains(out.String(), "全部") || !strings.Contains(out.String(), "语言 / Language") {
		t.Errorf("menu not in Chinese with the bilingual language item:\n%s", out.String())
	}
	// from the menu: item 5, then 日本語 (4); the menu comes back in Japanese
	out.Reset()
	got = wizard(strings.NewReader("5\n4\n1\n"+ip+"\n\n"), &out, l, false)
	if len(got) != 1 || lang != "ja" || loadConfig().Lang != "ja" || !strings.Contains(out.String(), "言語 / Language") {
		t.Fatalf("menu change: runs %q, lang %s, saved %q", got, lang, loadConfig().Lang)
	}
}

func TestTextWidth(t *testing.T) {
	if textWidth("中位数") != 6 || textWidth("Median") != 6 || textWidth("\x1b[32mT0\x1b[0m") != 2 || textWidth("±0.2") != 4 {
		t.Error("widths")
	}
}

func TestTranslationsComplete(t *testing.T) {
	for _, l := range languages {
		for k := range texts["en"] {
			if texts[l.Code][k] == "" {
				t.Errorf("%s: missing %s", l.Code, k)
			}
		}
		if len(continentText[l.Code]) != len(continentText["en"]) {
			t.Errorf("%s: continents", l.Code)
		}
	}
}
