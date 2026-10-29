package main

import (
	"io"
	"reflect"
	"strings"
	"testing"
)

// keys builds a key sequence; the IP answer (marked ipKey) is only typed where the machine has IPv6, since the IP
// question is skipped otherwise.
const ipKey = "<ip>"

func keys(v6 bool, parts ...string) string {
	var b strings.Builder
	for _, p := range parts {
		if strings.HasPrefix(p, ipKey) {
			if !v6 {
				continue
			}
			p = strings.TrimPrefix(p, ipKey)
		}
		b.WriteString(p + "\n")
	}
	// the last step, saving a file, keeps its default
	return b.String() + "\n"
}

func TestWizardSave(t *testing.T) {
	l, _ := parseList(embeddedList)
	k := keys(hasRoute(6), "1", ipKey, "", "")
	got := wizard(strings.NewReader(k[:len(k)-1]+"3\n"), io.Discard, l, false)
	if !reflect.DeepEqual(got, []string{"-o", "json"}) {
		t.Errorf("got %q", got)
	}
}

func TestWizard(t *testing.T) {
	l, err := parseList(embeddedList)
	if err != nil {
		t.Fatal(err)
	}
	v6 := hasRoute(6)
	cases := []struct {
		name string
		keys []string
		want []string
	}{
		{"all defaults", []string{"", ipKey, "", ""}, []string{}},
		{"nearest, https, quick", []string{"2", ipKey, "2", "1"}, []string{"--near", "20", "-m", "https", "-n", "5"}},
		{"two continents, icmp, precise", []string{"4", "1,2", ipKey, "3", "3"}, []string{"--continent", "as,eu", "-m", "icmp", "-n", "20"}},
		{"retry after bad input", []string{"9", "x", "1", ipKey, "4", ""}, []string{"-m", "all"}},
		{"0 on the mode question goes back", []string{"1", ipKey, "0", ipKey, "2", "", "", ""}, nil}, // filled below
		{"0 in the provider list goes back", []string{"3", "0", "2", ipKey, "", ""}, []string{"--near", "20"}},
	}
	for _, c := range cases {
		want := c.want
		if c.name == "0 on the mode question goes back" {
			// with IPv6, 0 on the mode question returns to the IP question (answered "2" = IPv6 only);
			// without, it returns to the first question (answered "2" = nearest)
			if v6 {
				c.keys = []string{"1", "", "0", "2", "", ""}
				want = []string{"--ip", "6"}
			} else {
				c.keys = []string{"1", "0", "2", "", ""}
				want = []string{"--near", "20"}
			}
		}
		got := wizard(strings.NewReader(keys(v6, c.keys...)), io.Discard, l, false)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %q, want %q", c.name, got, want)
		}
	}
}

func TestWizardBoth(t *testing.T) {
	if !hasRoute(6) {
		t.Skip("no IPv6 here: the IP question is not asked")
	}
	l, _ := parseList(embeddedList)
	got := wizard(strings.NewReader("1\n3\n\n\n\n"), io.Discard, l, false)
	if !reflect.DeepEqual(got, []string{"--ip", "both"}) {
		t.Errorf("got %q", got)
	}
}

func TestWizardExit(t *testing.T) {
	l, _ := parseList(embeddedList)
	if got := wizard(strings.NewReader("0\n"), io.Discard, l, false); got != nil {
		t.Errorf("0 on the first question should exit, got %q", got)
	}
	if got := wizard(strings.NewReader(""), io.Discard, l, false); got != nil {
		t.Errorf("closed input should end the menu, got %q", got)
	}
}

func TestWizardProviders(t *testing.T) {
	l, _ := parseList(embeddedList)
	got := wizard(strings.NewReader(keys(hasRoute(6), "3", "1,2", ipKey, "", "")), io.Discard, l, false)
	if len(got) != 2 || got[0] != "-p" || strings.Count(got[1], ",") != 1 {
		t.Fatalf("got %q", got)
	}
	// the two largest providers come first
	if p := got[1]; !strings.Contains(p, "oracle") || !strings.Contains(p, "gcp") {
		t.Errorf("expected the two largest providers, got %s", p)
	}
}

func TestCommandLine(t *testing.T) {
	if got := commandLine([]string{"-r", "ap-*", "--ip", "both"}); got != "vps2cloud -r 'ap-*' --ip both" {
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
	v6 := hasRoute(6)
	// first run: pick 简体中文 (2), then the defaults; the choice is saved
	lang = "en"
	var out strings.Builder
	got := wizard(strings.NewReader(keys(v6, "2", "", ipKey, "", "")), &out, l, true)
	if got == nil || lang != "zh-CN" || loadConfig().Lang != "zh-CN" {
		t.Fatalf("first run: got %q, lang %s, saved %q", got, lang, loadConfig().Lang)
	}
	if !strings.Contains(out.String(), "全部") || !strings.Contains(out.String(), "语言 / Language") || !strings.Contains(out.String(), "0) 退出") {
		t.Errorf("menu not in Chinese with the bilingual language item and exit:\n%s", out.String())
	}
	// from the menu: item 5, then 日本語 (4); the menu comes back in Japanese
	out.Reset()
	got = wizard(strings.NewReader(keys(v6, "5", "4", "1", ipKey, "", "")), &out, l, false)
	if got == nil || lang != "ja" || loadConfig().Lang != "ja" || !strings.Contains(out.String(), "言語 / Language") {
		t.Fatalf("menu change: got %q, lang %s, saved %q", got, lang, loadConfig().Lang)
	}
	// 0 in the language list goes back without changing it
	out.Reset()
	got = wizard(strings.NewReader(keys(v6, "5", "0", "1", ipKey, "", "")), &out, l, false)
	if got == nil || lang != "ja" {
		t.Fatalf("back from the language list: got %q, lang %s", got, lang)
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
