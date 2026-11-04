package main

import (
	"io"
	"reflect"
	"strconv"
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
	got := wizard(strings.NewReader(k[:len(k)-1]+"3\n"), io.Discard, l, false, nil)
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
		{"all regions, https, quick", []string{"1", ipKey, "2", "1"}, []string{"-m", "https", "-n", "5"}},
		{"two continents, icmp, precise", []string{"3", "1,2", ipKey, "3", "3"}, []string{"--continent", "as,eu", "-m", "icmp", "-n", "20"}},
		{"retry after bad input", []string{"9", "x", "1", ipKey, "4", ""}, []string{"-m", "all"}},
		{"0 on the mode question goes back", nil, nil}, // filled below
		{"0 in the provider list goes back", []string{"2", "0", "1", ipKey, "", ""}, []string{}},
	}
	for _, c := range cases {
		want := c.want
		if c.name == "0 on the mode question goes back" {
			// with IPv6, 0 on the mode question returns to the IP question (answered "2" = IPv6 only);
			// without, it returns to the first question (answered "1" = all), and the mode is then HTTPS
			if v6 {
				c.keys = []string{"1", "", "0", "2", "", ""}
				want = []string{"--ip", "6"}
			} else {
				c.keys = []string{"1", "0", "1", "2", ""}
				want = []string{"-m", "https"}
			}
		}
		got := wizard(strings.NewReader(keys(v6, c.keys...)), io.Discard, l, false, nil)
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
	got := wizard(strings.NewReader("1\n3\n\n\n\n"), io.Discard, l, false, nil)
	if !reflect.DeepEqual(got, []string{"--ip", "both"}) {
		t.Errorf("got %q", got)
	}
}

func TestWizardExit(t *testing.T) {
	l, _ := parseList(embeddedList)
	if got := wizard(strings.NewReader("0\n"), io.Discard, l, false, nil); got != nil {
		t.Errorf("0 on the first question should exit, got %q", got)
	}
	if got := wizard(strings.NewReader(""), io.Discard, l, false, nil); got != nil {
		t.Errorf("closed input should end the menu, got %q", got)
	}
}

func TestWizardProviders(t *testing.T) {
	l, _ := parseList(embeddedList)
	got := wizard(strings.NewReader(keys(hasRoute(6), "2", "1,2", ipKey, "", "")), io.Discard, l, false, nil)
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
	got := wizard(strings.NewReader(keys(v6, "2", "", ipKey, "", "")), &out, l, true, nil)
	if got == nil || lang != "zh-CN" || loadConfig().Lang != "zh-CN" {
		t.Fatalf("first run: got %q, lang %s, saved %q", got, lang, loadConfig().Lang)
	}
	if !strings.Contains(out.String(), "全部") || !strings.Contains(out.String(), "语言 / Language") || !strings.Contains(out.String(), "0) 退出") {
		t.Errorf("menu not in Chinese with the bilingual language item and exit:\n%s", out.String())
	}
	// from the menu: item 5, then 日本語 (4); the menu comes back in Japanese
	out.Reset()
	got = wizard(strings.NewReader(keys(v6, "5", "4", "1", ipKey, "", "")), &out, l, false, nil)
	if got == nil || lang != "ja" || loadConfig().Lang != "ja" || !strings.Contains(out.String(), "言語 / Language") {
		t.Fatalf("menu change: got %q, lang %s, saved %q", got, lang, loadConfig().Lang)
	}
	// 0 in the language list goes back without changing it
	out.Reset()
	got = wizard(strings.NewReader(keys(v6, "5", "0", "1", ipKey, "", "")), &out, l, false, nil)
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
			if texts[l.Code][k] == "" && pluralBase(k) == "" {
				t.Errorf("%s: missing %s", l.Code, k)
			}
		}
		// a plural form belongs to a text of that language, and only languages with that form have it
		for k := range texts[l.Code] {
			if b := pluralBase(k); b != "" {
				if texts[l.Code][b] == "" {
					t.Errorf("%s: %s without %s", l.Code, k, b)
				}
				if f := k[len(b)+1:]; (f == "few" || f == "many") && l.Code != "ru" || f == "one" && (l.Code == "zh-CN" || l.Code == "zh-TW" || l.Code == "ja" || l.Code == "ko") {
					t.Errorf("%s: %s has no %s form", l.Code, k, f)
				}
			}
		}
		if len(continentText[l.Code]) != len(continentText["en"]) {
			t.Errorf("%s: continents", l.Code)
		}
	}
}

func TestWizardCDN(t *testing.T) {
	tempConfig(t)
	l, _ := parseList(embeddedList)
	if len(l.CDNs) == 0 {
		t.Fatal("the built-in list has no CDNs")
	}
	// item 4 goes straight on to the IP version and samples: CDNs have no method to choose
	var out strings.Builder
	got := wizard(strings.NewReader(keys(hasRoute(6), "4", ipKey, "")), &out, l, false, nil)
	if len(got) != 1 || got[0] != "--cdn" || strings.Contains(out.String(), T("how")) {
		t.Fatalf("got %q:\n%s", got, out.String())
	}
}

func TestWizardLast(t *testing.T) {
	tempConfig(t)
	l, _ := parseList(embeddedList)
	last := []string{"-p", "aws,gcp", "-n", "5"}
	// with a previous run, it is the first item and the default: Enter runs it again
	var out strings.Builder
	got := wizard(strings.NewReader("\n"), &out, l, false, last)
	if strings.Join(got, " ") != strings.Join(last, " ") || !strings.Contains(out.String(), "-p aws,gcp") {
		t.Fatalf("got %q:\n%s", got, out.String())
	}
	// the other items move down one: 2 is now "all regions", and the language item is still the last
	got = wizard(strings.NewReader(keys(hasRoute(6), "2", ipKey, "", "", "")), io.Discard, l, false, last)
	if len(got) != 0 {
		t.Fatalf("item 2 with a last run: got %q, want the all-regions default", got)
	}
	n := 6
	if len(l.CDNs) == 0 {
		n = 5
	}
	out.Reset()
	got = wizard(strings.NewReader(keys(hasRoute(6), strconv.Itoa(n), "0", "2", ipKey, "", "", "")), &out, l, false, last)
	if !strings.Contains(out.String(), "Language") {
		t.Fatalf("item %d should open the language menu:\n%s", n, out.String())
	}
	// without one, nothing moves
	got = wizard(strings.NewReader(keys(hasRoute(6), "1", ipKey, "", "", "")), io.Discard, l, false, nil)
	if len(got) != 0 {
		t.Fatalf("no last run: got %q", got)
	}
}

// pluralBase is the text a plural form belongs to ("answered" for "answered_one"), or "" for a text that is not one.
func pluralBase(k string) string {
	for _, f := range []string{"_one", "_few", "_many"} {
		if strings.HasSuffix(k, f) {
			return strings.TrimSuffix(k, f)
		}
	}
	return ""
}

func TestPlural(t *testing.T) {
	defer func(l string) { lang = l }(lang)
	cases := []struct {
		lang string
		n    int
		want string
	}{{"en", 1, "one"}, {"en", 0, "other"}, {"en", 2, "other"}, {"fr", 0, "one"}, {"fr", 1, "one"}, {"fr", 2, "other"}, {"pt", 0, "one"},
		{"de", 1, "one"}, {"es", 21, "other"}, {"zh-CN", 1, "other"}, {"ja", 1, "other"}, {"ko", 1, "other"},
		{"ru", 1, "one"}, {"ru", 21, "one"}, {"ru", 101, "one"}, {"ru", 11, "many"}, {"ru", 2, "few"}, {"ru", 4, "few"}, {"ru", 24, "few"},
		{"ru", 12, "many"}, {"ru", 14, "many"}, {"ru", 5, "many"}, {"ru", 0, "many"}, {"ru", 88, "many"}, {"ru", 112, "many"}}
	for _, c := range cases {
		lang = c.lang
		if got := plural(c.n); got != c.want {
			t.Errorf("%s %d: got %s, want %s", c.lang, c.n, got, c.want)
		}
	}
	// TN picks the form, and falls back to the plain text where a language has no form for that rule
	lang = "ru"
	for n, want := range map[int]string{1: "1 замер", 3: "3 замера", 5: "5 замеров", 21: "21 замер", 22: "22 замера"} {
		if got := TN("nSamples", n, n); got != want {
			t.Errorf("ru %d: %q", n, got)
		}
	}
	lang = "en"
	if TN("nSamples", 1, 1) != "1 sample" || TN("nSamples", 2, 2) != "2 samples" || TN("more", 1, 1) != T("more", 1) || TN("fitOne", 1, 1, "gaming") != "1 region fits gaming" || TN("answered", 1, 1, 1, "") != "1 region of 1 answered" {
		t.Error("en forms")
	}
	lang = "zh-CN"
	if TN("nSamples", 1, 1) != "每个 1 次" {
		t.Error("zh-CN has one form only")
	}
}
