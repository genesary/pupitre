package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/httprate"
)

// limitByEmail limits the requests that name the same email to requests
// per window, as the per-address limit does for an address: guessing one
// account's password from many addresses is limited too
// (docs/specs/190-backend-defects.md B12). A request without an email in
// its JSON body (the SSO callbacks) is not counted.
func limitByEmail(requests int, window time.Duration) func(http.Handler) http.Handler {
	limiter := httprate.NewRateLimiter(requests, window)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
			email := bodyEmail(req)
			if email != "" && limiter.RespondOnLimit(writer, req, "email:"+email) {
				return
			}

			next.ServeHTTP(writer, req)
		})
	}
}

// bodyEmail reads the "email" field of a JSON body, normalized, and puts
// the body back for the handler. The body is already capped at 1 MB.
func bodyEmail(req *http.Request) string {
	if req.Body == nil {
		return ""
	}

	data, err := io.ReadAll(req.Body)
	req.Body = io.NopCloser(bytes.NewReader(data))

	if err != nil {
		return ""
	}

	var body struct {
		Email string `json:"email"`
	}

	if json.Unmarshal(data, &body) != nil {
		return ""
	}

	return strings.ToLower(strings.TrimSpace(body.Email))
}
