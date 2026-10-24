package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type fixtureTransport func(*http.Request) (*http.Response, error)

func (f fixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type protocolFixture struct {
	URL   string
	Close func()
}

func newProtocolFixture(t *testing.T, handler http.Handler) *protocolFixture {
	t.Helper()
	old, oldAdmin := SafeProxyHTTPClient().Transport, adminModelHTTPClient.Transport
	transport := fixtureTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "upstream.invalid" {
			t.Fatalf("unexpected fixture destination %s", r.URL.Host)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Result(), nil
	})
	SafeProxyHTTPClient().Transport = transport
	adminModelHTTPClient.Transport = transport
	return &protocolFixture{URL: "https://upstream.invalid", Close: func() { SafeProxyHTTPClient().Transport = old; adminModelHTTPClient.Transport = oldAdmin }}
}
