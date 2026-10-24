package service

import (
	"context"
	"golang.org/x/net/dns/dnsmessage"
	"net"
	"net/http"
	"testing"
)

func TestIsBlockedProxyIP(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1":       true,
		"10.0.0.1":        true,
		"172.16.0.1":      true,
		"192.168.1.1":     true,
		"169.254.169.254": true,
		"100.100.100.200": true,
		"0.0.0.0":         true,
		"::1":             true,
		"fc00::1":         true,
		"fe80::1":         true,
		"1.1.1.1":         false,
		"8.8.8.8":         false,
	}
	for raw, expected := range cases {
		actual := isBlockedProxyIP(net.ParseIP(raw))
		if actual != expected {
			t.Fatalf(
				"isBlockedProxyIP(%q) = %v，期望 %v",
				raw,
				actual,
				expected,
			)
		}
	}
}

func TestSafeProxyHTTPClientRedirectLimit(t *testing.T) {
	client := SafeProxyHTTPClient()
	request, err := http.NewRequest(
		http.MethodGet,
		"https://example.com/image.png",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CheckRedirect(
		request,
		make([]*http.Request, 5),
	); err != nil {
		t.Fatalf("第 5 次重定向不应被拒绝：%v", err)
	}
	if err := client.CheckRedirect(
		request,
		make([]*http.Request, 6),
	); err == nil {
		t.Fatal("超过 5 次重定向应被拒绝")
	}
}
func TestOutboundPolicyBlockedDestinations(t *testing.T) {
	for _, raw := range []string{"http://127.0.0.1:1/", "http://localhost:1/", "http://[::1]:1/", "http://169.254.169.254/", "http://100.100.100.200/", "http://[::ffff:127.0.0.1]/", "http://[64:ff9b::7f00:1]/", "http://user:secret@example.com/"} {
		t.Run(raw, func(t *testing.T) {
			if response, err := newSafeProxyHTTPClient().Get(raw); err == nil {
				response.Body.Close()
				t.Fatal("forbidden address accepted")
			}
		})
	}
}
func TestOutboundRedirectBoundary(t *testing.T) {
	original, _ := http.NewRequest("POST", "https://one.invalid/task", nil)
	next, _ := http.NewRequest("POST", "https://two.invalid/task", nil)
	next.Header.Set("X-Custom-Secret", "key")
	client := newSafeProxyHTTPClient()
	if client.CheckRedirect(next, []*http.Request{original}) == nil {
		t.Fatal("cross-origin body forwarded")
	}
	next.Method = "GET"
	next.Header.Set("Authorization", "Bearer key")
	next.Header.Set("Range", "bytes=0-10")
	if err := client.CheckRedirect(next, []*http.Request{original}); err != nil {
		t.Fatal(err)
	}
	if next.Header.Get("Authorization") != "" || next.Header.Get("X-Custom-Secret") != "" || next.Header.Get("Range") == "" {
		t.Fatal("redirect credential leak")
	}
	internal, _ := http.NewRequest("GET", "http://169.254.169.254/", nil)
	if client.CheckRedirect(internal, []*http.Request{original}) == nil {
		t.Fatal("metadata redirect accepted")
	}
}

func TestConnectTimeDNSRejectsMixedPublicPrivateAnswers(t *testing.T) {
	listener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		buf := make([]byte, 2048)
		for {
			n, peer, e := listener.ReadFrom(buf)
			if e != nil {
				return
			}
			var query dnsmessage.Message
			if query.Unpack(buf[:n]) != nil {
				continue
			}
			response := dnsmessage.Message{Header: dnsmessage.Header{ID: query.ID, Response: true, RecursionDesired: true, RecursionAvailable: true}, Questions: query.Questions}
			for _, q := range query.Questions {
				if q.Type == dnsmessage.TypeA {
					for _, ip := range [][4]byte{{1, 1, 1, 1}, {127, 0, 0, 1}} {
						response.Answers = append(response.Answers, dnsmessage.Resource{Header: dnsmessage.ResourceHeader{Name: q.Name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET, TTL: 0}, Body: &dnsmessage.AResource{A: ip}})
					}
				}
			}
			packet, e := response.Pack()
			if e == nil {
				listener.WriteTo(packet, peer)
			}
		}
	}()
	old := net.DefaultResolver
	defer func() { net.DefaultResolver = old }()
	net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "udp", listener.LocalAddr().String())
	}}
	if response, err := newSafeProxyHTTPClient().Get("http://rebind.invalid/file"); err == nil {
		response.Body.Close()
		t.Fatal("mixed DNS answer accepted")
	}
}
