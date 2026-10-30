package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"0.4.0", "0.3.1", true},
		{"0.10.0", "0.9.9", true},
		{"1.0.0", "0.99.0", true},
		{"0.3.1", "0.3.1", false},
		{"0.3.0", "0.3.1", false},
		{"0.4.0", "dev", false},
		{"0.4.0-rc1", "0.3.0", false},
		{"", "0.3.0", false},
	} {
		if got := newer(c.a, c.b); got != c.want {
			t.Errorf("newer(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

func TestJustLooking(t *testing.T) {
	for _, c := range []struct {
		args []string
		want bool
	}{
		{[]string{"--version"}, true},
		{[]string{"-v"}, true},
		{[]string{"-h"}, true},
		{[]string{"--help"}, true},
		{[]string{"--list", "-p", "aws"}, true},
		{[]string{"-p", "aws"}, false},
		{[]string{"-r", "list"}, false},
		{nil, false},
	} {
		if got := justLooking(c.args); got != c.want {
			t.Errorf("justLooking(%q) = %v", c.args, got)
		}
	}
}

func TestLatestVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" || r.URL.Path != "/releases/latest" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		http.Redirect(w, r, "/releases/tag/v0.4.2", http.StatusFound)
	}))
	defer srv.Close()
	old := releaseURL
	releaseURL = srv.URL + "/releases"
	defer func() { releaseURL = old }()
	v, err := latestVersion("test")
	if err != nil || v != "0.4.2" {
		t.Fatalf("latestVersion = %q, %v", v, err)
	}
}
