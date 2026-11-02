package handlers

// checker-service is gone: the monolith calls RunEvaluate and RunCheckStep
// directly (docs/specs/107-backend-labs-and-checker.md). This file keeps,
// for the tests only, checker-service's router and HTTP handlers, as thin
// shims over those methods, so the handler tests keep their assertions.

import (
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"go.uber.org/zap"

	"github.com/jojo91550/pupitre2/apps/api/internal/checker"
	"github.com/jojo91550/pupitre2/apps/api/internal/platform/httperr"
)

// maxRequestBodyBytes caps the size of accepted request bodies (1 MB).
const maxRequestBodyBytes = 1 << 20

// remoteIP extracts the IP from r.RemoteAddr (already set to the real client
// IP by chiMiddleware.RealIP) to use as the rate-limit key.
func remoteIP(r *http.Request) (string, error) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr, nil //nolint:nilerr // best-effort: fall back to raw addr
	}

	return ip, nil
}

// BuildRouter builds the chi router for the checker-service HTTP API.
func (h *Handler) BuildRouter() *chi.Mux {
	router := chi.NewRouter()
	router.Use(chiMiddleware.RequestID)
	//nolint:staticcheck // SA1019: RealIP deprecation acknowledged — service runs behind a trusted reverse proxy that sets X-Forwarded-For; the value is used for logging/rate-limiting only.
	router.Use(chiMiddleware.RealIP)
	router.Use(chiMiddleware.Logger)
	router.Use(chiMiddleware.Recoverer)
	router.Use(chiMiddleware.RequestSize(maxRequestBodyBytes))

	if h.config.RateLimitRequests > 0 {
		router.Use(httprate.LimitBy(h.config.RateLimitRequests, time.Duration(h.config.RateLimitWindowSeconds)*time.Second, remoteIP))
	}

	router.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		if err != nil {
			zap.L().Error("encode health response", zap.Error(err))
		}
	})

	router.Post("/evaluate", h.Evaluate)
	router.Post("/check-step", h.CheckStep)

	return router
}

// Evaluate handles POST /evaluate: it fetches the Rego policy from the
// course git repository, fetches GitLab state for the request, then
// precompiles and evaluates the policy with restricted OPA capabilities.
func (h *Handler) Evaluate(resp http.ResponseWriter, httpReq *http.Request) {
	httpReq.Body = http.MaxBytesReader(resp, httpReq.Body, maxRequestBodyBytes)

	req, valid := decodeEvaluateRequest(resp, httpReq)
	if !valid {
		return // error response and log written by callee
	}

	result, fail := h.RunEvaluate(httpReq.Context(), req)
	writeResult(resp, result, fail, "encode evaluate response")
}

// CheckStep handles POST /check-step: runs a single named GitLab check
// without a full OPA policy evaluation.
func (h *Handler) CheckStep(resp http.ResponseWriter, httpReq *http.Request) {
	var req checker.StepRequest

	err := json.NewDecoder(httpReq.Body).Decode(&req)
	if err != nil {
		httperr.Write(resp, http.StatusBadRequest, "invalid request body")

		return
	}

	result, fail := h.RunCheckStep(req)
	writeResult(resp, result, fail, "encode check-step response")
}

// writeResult writes a check result, or the failure that replaced it.
func writeResult(resp http.ResponseWriter, result *checker.EvaluateResponse, fail *Failure, logMsg string) {
	if fail != nil {
		httperr.Write(resp, fail.Status, fail.Message)

		return
	}

	resp.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(resp).Encode(result)
	if err != nil {
		zap.L().Error(logMsg, zap.Error(err))
	}
}

// decodeEvaluateRequest decodes and validates the JSON body of POST /evaluate.
// It writes an appropriate error response and returns false on failure.
func decodeEvaluateRequest(resp http.ResponseWriter, httpReq *http.Request) (checker.EvaluateRequest, bool) {
	var req checker.EvaluateRequest

	err := json.NewDecoder(httpReq.Body).Decode(&req)
	if err != nil {
		httperr.Write(resp, http.StatusBadRequest, "invalid request body")

		return checker.EvaluateRequest{}, false
	}

	if req.Username == "" || req.Project == "" {
		httperr.Write(resp, http.StatusBadRequest, "username and project are required")

		return checker.EvaluateRequest{}, false
	}

	if req.PolicySrc == "" || req.PolicyRef == "" || req.PolicyPath == "" {
		httperr.Write(resp, http.StatusBadRequest, "policySrc, policyRef and policyPath are required")

		return checker.EvaluateRequest{}, false
	}

	return req, true
}
