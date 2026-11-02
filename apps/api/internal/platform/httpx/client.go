// Package httpx provides the tuned HTTP client this service uses for every
// outbound call, plus small helpers for the JSON request/response shape all
// of those calls share.
//
// It exists because [http.DefaultClient] is the wrong client for
// service-to-service traffic on two counts, both of which only show up
// under load:
//
//   - It has no timeout at all. One hung dependency is enough to pin every
//     goroutine that talks to it until the peer gives up, which under load
//     means the whole server.
//   - [http.DefaultTransport] allows two idle connections per host. Past two
//     concurrent calls to the same peer, every extra request opens a fresh
//     TCP (and TLS) connection and throws it away afterwards, so a busy
//     service spends its time in connection setup and fills the local
//     table with sockets in TIME_WAIT.
package httpx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

const (
	// DefaultTimeout bounds a whole outbound call — connect, write,
	// response headers and body. Internal calls are single-digit
	// milliseconds when healthy, so this only ever fires on a peer that
	// has stopped answering.
	DefaultTimeout = 10 * time.Second

	// maxIdleConns is the total idle keep-alive pool size across all peers.
	maxIdleConns = 256

	// maxIdleConnsPerHost is the idle pool size per peer. A service only
	// ever talks to a couple of peers, so this is deliberately the same
	// order as maxIdleConns: the pool should absorb a burst of concurrent
	// calls to one peer rather than churning connections.
	maxIdleConnsPerHost = 128

	// idleConnTimeout is how long an unused keep-alive connection is held
	// open before being closed.
	idleConnTimeout = 90 * time.Second

	// dialTimeout bounds establishing the TCP connection.
	dialTimeout = 5 * time.Second

	// keepAliveInterval is the TCP keep-alive probe interval, which lets a
	// silently-dropped connection be noticed rather than hanging.
	keepAliveInterval = 30 * time.Second

	// tlsHandshakeTimeout bounds the TLS handshake.
	tlsHandshakeTimeout = 5 * time.Second

	// expectContinueTimeout bounds the wait for a 100-continue response.
	expectContinueTimeout = 1 * time.Second

	// responseHeaderTimeout bounds the wait for response headers after the
	// request has been written, so a peer that accepts a connection and
	// then stalls is detected well before DefaultTimeout.
	responseHeaderTimeout = 8 * time.Second

	// maxErrorBodyBytes caps how much of an error response body is read
	// into the returned error message.
	maxErrorBodyBytes = 4 << 10
)

// NewTransport builds the tuned transport shared by this service's clients.
func NewTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   dialTimeout,
			KeepAlive: keepAliveInterval,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          maxIdleConns,
		MaxIdleConnsPerHost:   maxIdleConnsPerHost,
		IdleConnTimeout:       idleConnTimeout,
		TLSHandshakeTimeout:   tlsHandshakeTimeout,
		ExpectContinueTimeout: expectContinueTimeout,
		ResponseHeaderTimeout: responseHeaderTimeout,
	}
}

// New builds a client with the tuned transport and the given whole-call
// timeout. A timeout of zero or less uses DefaultTimeout.
func New(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	return &http.Client{
		Transport: NewTransport(),
		Timeout:   timeout,
	}
}

// shared is the process-wide client used by Client. One client — and so
// one connection pool — is the point: a per-call client would defeat
// keep-alive entirely.
//
//nolint:gochecknoglobals // a single shared connection pool is the reason this package exists
var shared = New(DefaultTimeout)

// Client returns the process-wide tuned HTTP client.
func Client() *http.Client {
	return shared
}

// Do sends req on the shared client.
//
// The caller owns the returned response and must pass it to [Drain].
func Do(req *http.Request) (*http.Response, error) {
	resp, err := shared.Do(req) //nolint:gosec // callers build URLs from trusted service config, never from request input
	if err != nil {
		return nil, fmt.Errorf("http %s %s: %w", req.Method, req.URL.Redacted(), err)
	}

	return resp, nil
}

// GetJSON performs a GET on rawURL, applying decorate to the request
// before sending it, and decodes a 2xx JSON body into dest.
//
// The body is always drained and closed: a response body that is closed
// without being read cannot be reused by the keep-alive pool, which
// quietly turns a pooled client back into a connection-per-request one.
func GetJSON(ctx context.Context, rawURL string, decorate func(*http.Request), dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("build GET %s: %w", rawURL, err)
	}

	if decorate != nil {
		decorate(req)
	}

	resp, err := Do(req)
	if err != nil {
		return err
	}
	defer Drain(resp)

	if resp.StatusCode != http.StatusOK {
		return statusError(rawURL, resp)
	}

	err = json.NewDecoder(resp.Body).Decode(dest)
	if err != nil {
		return fmt.Errorf("decode %s: %w", rawURL, err)
	}

	return nil
}

// Drain reads any unread remainder of resp's body and closes it, so the
// underlying connection returns to the idle pool instead of being dropped.
func Drain(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}

	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorBodyBytes))
	_ = resp.Body.Close()
}

// statusError builds the error reported for a non-200 response, including
// a bounded prefix of the body.
func statusError(rawURL string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))

	return fmt.Errorf("GET %s returned %d: %s", rawURL, resp.StatusCode, body)
}
