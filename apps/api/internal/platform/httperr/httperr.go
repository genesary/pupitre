// Package httperr writes the JSON error envelope shared by every pupitre
// HTTP service, so that a client sees the same error shape regardless of
// which service answered.
package httperr

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"
)

// Write emits a JSON error response with the given status and message. An
// encoding failure is logged rather than returned: the status line has
// already been written, so there is nothing left to signal to the client.
func Write(resp http.ResponseWriter, status int, msg string) {
	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(status)

	err := json.NewEncoder(resp).Encode(map[string]string{"error": msg})
	if err != nil {
		zap.L().Error("encode error response", zap.Error(err))
	}
}
