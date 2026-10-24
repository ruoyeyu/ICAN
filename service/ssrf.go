package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

var safeProxyHTTPClient = newSafeProxyHTTPClient()

func SafeProxyHTTPClient() *http.Client {
	return safeProxyHTTPClient
}

// OutboundHTTPClient keeps the existing operation timeout while sharing the
// connect-time IP policy. User supplied URLs never use ambient HTTP proxies.
func OutboundHTTPClient(timeout time.Duration) *http.Client {
	client := *SafeProxyHTTPClient()
	client.Timeout = timeout
	return &client
}

func ValidateOutboundURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return errors.New("外部地址必须为不含凭据的 HTTP(S) 地址")
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && isBlockedProxyIP(ip) {
		return errors.New("禁止访问本地或内网地址")
	}
	return nil
}
func newSafeProxyHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = safeProxyDialContext
	return &http.Client{
		Transport: publicTransport{transport},
		Timeout:   5 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 5 {
				return errors.New("重定向次数过多")
			}
			if err := ValidateOutboundURL(req.URL.String()); err != nil {
				return err
			}
			if len(via) > 0 && via[0] != nil && via[0].URL != nil && !strings.EqualFold(req.URL.Scheme+"://"+req.URL.Host, via[0].URL.Scheme+"://"+via[0].URL.Host) {
				if req.Method != http.MethodGet && req.Method != http.MethodHead {
					return errors.New("禁止跨来源转发请求正文")
				}
				for key := range req.Header {
					if key != "Accept" && key != "Range" && key != "User-Agent" {
						req.Header.Del(key)
					}
				}
			}
			return nil
		},
	}
}

func safeProxyDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(ips) == 0 {
		return nil, errors.New("无法解析目标地址")
	}
	for _, item := range ips {
		if isBlockedProxyIP(item.IP) {
			return nil, errors.New("禁止访问本地或内网地址")
		}
	}
	dialer := net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	var lastErr error
	for _, item := range ips {
		conn, err := dialer.DialContext(
			ctx,
			network,
			net.JoinHostPort(item.String(), port),
		)
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("无法连接目标地址: %w", lastErr)
}

func isBlockedProxyIP(ip net.IP) bool {
	if !ip.IsGlobalUnicast() {
		return true
	}
	address, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	address = address.Unmap()
	// Special-purpose and transition ranges must not become a route to internal
	// services through IPv4 translation, documentation/reserved networks or metadata.
	for _, cidr := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "64:ff9b::/96", "64:ff9b:1::/48", "2001::/23", "2002::/16"} {
		if netip.MustParsePrefix(cidr).Contains(address) {
			return true
		}
	}
	if ip == nil {
		return true
	}
	if ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() {
		return true
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	// 100.64.0.0/10，包括部分云平台 metadata 地址。
	if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
		return true
	}
	return ip4[0] == 0
}

// The administrator may trust one storage destination. The exception is scoped
// to that exact origin, permits RFC1918/ULA only, and never permits metadata or
// loopback. The resolved address is pinned for each connection.
func TrustedStorageTransport(origin *url.URL) http.RoundTripper {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil || !strings.EqualFold(host, origin.Hostname()) {
			return nil, errors.New("存储目标不匹配")
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(ips) == 0 {
			return nil, errors.New("无法解析存储地址")
		}
		for _, item := range ips {
			if isBlockedProxyIP(item.IP) && !item.IP.IsPrivate() {
				return nil, errors.New("禁止访问本地或元数据地址")
			}
		}
		for _, item := range ips {
			conn, e := (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext(ctx, network, net.JoinHostPort(item.IP.String(), port))
			if e == nil {
				return conn, nil
			}
			err = e
		}
		return nil, err
	}
	return originTransport{origin: origin, transport: transport}
}

type originTransport struct {
	origin    *url.URL
	transport http.RoundTripper
}

func (t originTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.User != nil || !strings.EqualFold(r.URL.Scheme+"://"+r.URL.Host, t.origin.Scheme+"://"+t.origin.Host) {
		return nil, errors.New("禁止存储跨来源请求")
	}
	return t.transport.RoundTrip(r)
}

type publicTransport struct{ http.RoundTripper }

func (t publicTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if err := ValidateOutboundURL(r.URL.String()); err != nil {
		return nil, err
	}
	return t.RoundTripper.RoundTrip(r)
}
