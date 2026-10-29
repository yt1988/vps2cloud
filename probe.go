package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// Politeness toward providers, the same as Me2Cloud's: a pause between samples to one host, and a host is given up
// after this many failures in a row (its remaining samples count as lost).
const failStop = 3

type Probe struct {
	N       int
	Gap     time.Duration
	Timeout time.Duration
	UA      string
}

var lost = math.NaN()

func ms(t0 time.Time) float64 { return float64(time.Since(t0).Microseconds()) / 1000 }

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// sampleLoop runs one measurement n times, pausing between them, and pads the rest as lost once the host has failed
// failStop times in a row.
func (p Probe) sampleLoop(ctx context.Context, one func(i int) (float64, error)) ([]float64, error) {
	out := make([]float64, 0, p.N)
	var lastErr error
	fails := 0
	for i := 0; i < p.N; i++ {
		if i > 0 && !sleepCtx(ctx, p.Gap) {
			break
		}
		v, err := one(i)
		if err != nil {
			out = append(out, lost)
			lastErr = err
			if fails++; fails >= failStop {
				break
			}
			continue
		}
		fails = 0
		out = append(out, v)
	}
	for len(out) < p.N {
		out = append(out, lost)
	}
	return out, lastErr
}

func resolve(ctx context.Context, host string, fam int) (net.IP, error) {
	network := "ip4"
	if fam == 6 {
		network = "ip6"
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIP(ctx, network, host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no IPv%d address", fam)
	}
	return ips[0], nil
}

// TCP: the time to complete a handshake with the host's HTTPS port, then close. One network round trip, with no
// server work in it, which makes it the cleanest number for comparing regions.
func (p Probe) TCP(ctx context.Context, ip net.IP, port int) ([]float64, error) {
	addr := net.JoinHostPort(ip.String(), strconv.Itoa(port))
	return p.sampleLoop(ctx, func(int) (float64, error) {
		d := net.Dialer{Timeout: p.Timeout}
		t0 := time.Now()
		c, err := d.DialContext(ctx, "tcp", addr)
		if err != nil {
			return 0, err
		}
		v := ms(t0)
		c.Close()
		return v, nil
	})
}

// HTTPS: the same measurement as Me2Cloud in the browser. One uncounted request opens the connection; each sample is
// then a request on that open connection, timed until the response headers arrive (one round trip plus a little server
// time). It returns how many samples had to open a new connection.
func (p Probe) HTTPS(ctx context.Context, e Endpoint, fam int, ip net.IP) ([]float64, int, error) {
	raw := e.URL
	if fam == 6 && e.URL6 != "" {
		raw = e.URL6
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, 0, err
	}
	port := u.Port()
	if port == "" {
		port = "443"
	}
	addr := net.JoinHostPort(ip.String(), port)
	tr := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: p.Timeout}).DialContext(ctx, "tcp", addr)
		},
		TLSClientConfig:     &tls.Config{ServerName: u.Hostname()},
		TLSHandshakeTimeout: p.Timeout,
		ForceAttemptHTTP2:   true,
		MaxIdleConnsPerHost: 1,
		DisableCompression:  true,
		IdleConnTimeout:     30 * time.Second,
	}
	defer tr.CloseIdleConnections()
	cl := &http.Client{Transport: tr, Timeout: p.Timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	method := "GET"
	if e.Method == "HEAD" {
		method = "HEAD"
	}
	reconn := 0
	do := func(i int) (float64, bool, error) {
		// no cache-busting query: some hosts (Hetzner's speed test servers) drop the connection when a URL has one
		req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
		if err != nil {
			return 0, false, err
		}
		req.Header.Set("User-Agent", p.UA)
		req.Header.Set("Cache-Control", "no-cache")
		reused := false
		req = req.WithContext(httptrace.WithClientTrace(req.Context(), &httptrace.ClientTrace{GotConn: func(g httptrace.GotConnInfo) { reused = g.Reused }}))
		t0 := time.Now()
		res, err := cl.Do(req)
		if err != nil {
			return 0, false, err
		}
		v := ms(t0)
		io.Copy(io.Discard, io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		return v, reused, nil
	}
	if _, _, err := do(-1); err != nil {
		return nil, 0, err
	}
	s, err := p.sampleLoop(ctx, func(i int) (float64, error) {
		v, reused, err := do(i)
		if err == nil && !reused {
			reconn++
		}
		return v, err
	})
	return s, reconn, err
}

// ICMP echo. Linux and macOS allow it without root through "ping sockets" when net.ipv4.ping_group_range covers the
// user's group; otherwise it needs a raw socket (root or CAP_NET_RAW).
func icmpListen(fam int) (*icmp.PacketConn, bool, error) {
	unpriv, priv, any := "udp4", "ip4:icmp", "0.0.0.0"
	if fam == 6 {
		unpriv, priv, any = "udp6", "ip6:ipv6-icmp", "::"
	}
	if c, err := icmp.ListenPacket(unpriv, any); err == nil {
		return c, false, nil
	}
	c, err := icmp.ListenPacket(priv, any)
	if err != nil {
		return nil, false, err
	}
	return c, true, nil
}

func (p Probe) ICMP(ctx context.Context, ip net.IP) ([]float64, error) {
	fam, proto := 4, 1
	var reqType, replyType icmp.Type = ipv4.ICMPTypeEcho, ipv4.ICMPTypeEchoReply
	if ip.To4() == nil {
		fam, proto = 6, 58
		reqType, replyType = ipv6.ICMPTypeEchoRequest, ipv6.ICMPTypeEchoReply
	}
	c, priv, err := icmpListen(fam)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	var dst net.Addr = &net.UDPAddr{IP: ip}
	if priv {
		dst = &net.IPAddr{IP: ip}
	}
	// a ping socket gets its echo ID from the kernel and only sees its own replies; a raw socket sees every reply, so
	// those are matched by ID too
	id := rand.IntN(0xffff)
	buf := make([]byte, 1500)
	return p.sampleLoop(ctx, func(i int) (float64, error) {
		seq := i + 1
		b, err := (&icmp.Message{Type: reqType, Body: &icmp.Echo{ID: id, Seq: seq, Data: []byte("vps2cloud")}}).Marshal(nil)
		if err != nil {
			return 0, err
		}
		t0 := time.Now()
		if _, err := c.WriteTo(b, dst); err != nil {
			return 0, err
		}
		c.SetReadDeadline(t0.Add(p.Timeout))
		for {
			n, peer, err := c.ReadFrom(buf)
			if err != nil {
				return 0, errors.New("timeout")
			}
			m, err := icmp.ParseMessage(proto, buf[:n])
			if err != nil || m.Type != replyType {
				continue
			}
			echo, ok := m.Body.(*icmp.Echo)
			if !ok || echo.Seq != seq || (priv && echo.ID != id) || !samePeer(peer, ip) {
				continue
			}
			return ms(t0), nil
		}
	})
}

func samePeer(a net.Addr, ip net.IP) bool {
	switch x := a.(type) {
	case *net.UDPAddr:
		return x.IP.Equal(ip)
	case *net.IPAddr:
		return x.IP.Equal(ip)
	}
	return strings.HasPrefix(a.String(), ip.String())
}
