package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

// The endpoint list is Me2Cloud's (github.com/yt1988/me2cloud, public/endpoints.json): one list, verified weekly, shared
// by the browser tool and this one. A snapshot is built in, and each run fetches the latest unless --offline is set.
//
//go:embed endpoints.json
var embeddedList []byte

// listURLs are tried in order. The first is a static file on v2c.1ms.dog; requests for it are counted (by the User-Agent
// below, never by IP) to show how often the tool runs. The second is the same file on Me2Cloud's site.
var listURLs = []string{"https://v2c.1ms.dog/endpoints.json", "https://m2c.1ms.dog/endpoints.json"}

type Vendor struct {
	Name    string `json:"name"`
	Method  string `json:"method"`
	Regions int    `json:"regions"`
}

type Endpoint struct {
	ID        string `json:"id"`
	Vendor    string `json:"vendor"`
	Region    string `json:"region"`
	City      string `json:"city"`
	Country   string `json:"country"`
	URL       string `json:"url"`
	Method    string `json:"method"`
	URL6      string `json:"url6"`
	Continent string `json:"continent"`
	// optional, for this tool: false where the host drops ICMP echo; 0 means port 443
	ICMP    *bool `json:"icmp,omitempty"`
	TCPPort int   `json:"tcpPort,omitempty"`
}

type List struct {
	Schema    int               `json:"schema"`
	Verified  string            `json:"verified"`
	Vendors   map[string]Vendor `json:"vendors"`
	Endpoints []Endpoint        `json:"endpoints"`
	Source    string            `json:"-"`
}

func parseList(b []byte) (*List, error) {
	var l List
	if err := json.Unmarshal(b, &l); err != nil {
		return nil, err
	}
	if l.Schema != 1 || len(l.Endpoints) == 0 {
		return nil, fmt.Errorf("unsupported endpoint list (schema %d, %d endpoints)", l.Schema, len(l.Endpoints))
	}
	return &l, nil
}

// loadList reads --endpoints when given, else fetches the latest list, else falls back to the built-in snapshot.
func loadList(file string, offline bool, ua string) (*List, error) {
	if file != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		l, err := parseList(b)
		if err == nil {
			l.Source = file
		}
		return l, err
	}
	if !offline {
		for _, u := range listURLs {
			if l, err := fetchList(u, ua); err == nil {
				l.Source = u
				return l, nil
			}
		}
	}
	l, err := parseList(embeddedList)
	if err == nil {
		l.Source = "built-in snapshot"
	}
	return l, err
}

func fetchList(u, ua string) (*List, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	req.Header.Set("User-Agent", ua)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, errors.New(res.Status)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	return parseList(b)
}

// Host returns the hostname to probe for an IP family: the IPv6-specific URL when there is one.
func (e Endpoint) Host(fam int) string {
	u := e.URL
	if fam == 6 && e.URL6 != "" {
		u = e.URL6
	}
	p, err := url.Parse(u)
	if err != nil {
		return ""
	}
	return p.Hostname()
}

func (e Endpoint) Port() int {
	if e.TCPPort > 0 {
		return e.TCPPort
	}
	return 443
}

type Filter struct {
	Providers, Regions, Continents, Countries []string
}

func splitList(s string) []string {
	var out []string
	for _, x := range strings.Split(s, ",") {
		if x = strings.ToLower(strings.TrimSpace(x)); x != "" {
			out = append(out, x)
		}
	}
	return out
}

// Select applies the filter. Providers match an id or a name ("aws", "hetzner", "google cloud"); regions match the
// region code, the id or the city, with shell-style wildcards ("ap-*", "*tokyo*", "fra*").
func (l *List) Select(f Filter) ([]Endpoint, error) {
	for _, p := range f.Providers {
		if !l.knownVendor(p) {
			return nil, fmt.Errorf("unknown provider %q (run with --list to see them)", p)
		}
	}
	var out []Endpoint
	for _, e := range l.Endpoints {
		if len(f.Providers) > 0 && !anyMatch(f.Providers, func(p string) bool { return p == e.Vendor || p == strings.ToLower(l.Vendors[e.Vendor].Name) }) {
			continue
		}
		if len(f.Continents) > 0 && !anyMatch(f.Continents, func(c string) bool { return c == e.Continent }) {
			continue
		}
		if len(f.Countries) > 0 && !anyMatch(f.Countries, func(c string) bool { return c == strings.ToLower(e.Country) }) {
			continue
		}
		if len(f.Regions) > 0 && !anyMatch(f.Regions, func(g string) bool {
			for _, s := range []string{e.Region, e.ID, e.City} {
				if ok, _ := path.Match(g, strings.ToLower(s)); ok {
					return true
				}
			}
			return false
		}) {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

func (l *List) knownVendor(p string) bool {
	for id, v := range l.Vendors {
		if p == id || p == strings.ToLower(v.Name) {
			return true
		}
	}
	return false
}

func anyMatch(xs []string, f func(string) bool) bool {
	for _, x := range xs {
		if f(x) {
			return true
		}
	}
	return false
}
