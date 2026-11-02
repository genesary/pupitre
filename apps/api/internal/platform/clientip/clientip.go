// Package clientip finds the address of the client behind the reverse
// proxies the API trusts (docs/specs/190-backend-defects.md B12).
//
// chi's RealIP middleware took X-Forwarded-For from anyone: a client could
// pick its own address, and so a fresh rate-limit bucket for every request.
// This middleware reads the forwarding headers only when the direct peer is
// a trusted proxy, and walks X-Forwarded-For from the right, so a client can
// prepend whatever it likes without being believed.
package clientip

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strings"
)

// DefaultTrustedProxies are the networks a reverse proxy of the platform
// sits in: loopback and the private ranges of a cluster.
//
//nolint:gochecknoglobals // read-only defaults
var DefaultTrustedProxies = []string{
	"127.0.0.0/8", "::1/128",
	"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7",
}

// ParsePrefixes parses CIDR blocks, or single addresses, into prefixes.
func ParsePrefixes(values []string) ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0, len(values))

	for _, raw := range values {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		if !strings.Contains(raw, "/") {
			addr, err := netip.ParseAddr(raw)
			if err != nil {
				return nil, fmt.Errorf("trusted proxy %q: %w", raw, err)
			}

			prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))

			continue
		}

		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			return nil, fmt.Errorf("trusted proxy %q: %w", raw, err)
		}

		prefixes = append(prefixes, prefix.Masked())
	}

	return prefixes, nil
}

// Middleware sets r.RemoteAddr to the client's address: the peer itself,
// unless the peer is a trusted proxy, in which case the right-most address of
// X-Forwarded-For (or X-Real-IP) that is not a trusted proxy.
func Middleware(trusted []netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
			if client, ok := clientAddr(req, trusted); ok {
				req.RemoteAddr = client
			}

			next.ServeHTTP(writer, req)
		})
	}
}

// clientAddr resolves the client address of req; ok is false when it is the
// peer itself.
func clientAddr(req *http.Request, trusted []netip.Prefix) (string, bool) {
	peer, ok := parseAddr(req.RemoteAddr)
	if !ok || !isTrusted(peer, trusted) {
		return "", false
	}

	if forwarded := req.Header.Values("X-Forwarded-For"); len(forwarded) > 0 {
		hops := strings.Split(strings.Join(forwarded, ","), ",")
		for _, raw := range slices.Backward(hops) {
			hop, valid := parseAddr(strings.TrimSpace(raw))
			if !valid {
				return "", false
			}

			if !isTrusted(hop, trusted) {
				return hop.String(), true
			}
		}

		// Every hop is a trusted proxy: the first one is the client.
		first, valid := parseAddr(strings.TrimSpace(hops[0]))

		return first.String(), valid
	}

	if realIP, valid := parseAddr(strings.TrimSpace(req.Header.Get("X-Real-IP"))); valid {
		return realIP.String(), true
	}

	return "", false
}

// parseAddr parses an address with or without a port.
func parseAddr(raw string) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(raw)
	if err == nil {
		raw = host
	}

	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return netip.Addr{}, false
	}

	return addr.Unmap(), true
}

// isTrusted reports whether addr belongs to one of the trusted networks.
func isTrusted(addr netip.Addr, trusted []netip.Prefix) bool {
	for _, prefix := range trusted {
		if prefix.Contains(addr) {
			return true
		}
	}

	return false
}
