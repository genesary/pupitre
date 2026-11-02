package handlers

import "net/http"

// Health godoc
// @Summary  Liveness check
// @Tags     Health
// @Produce  json
// @Success  200  {object}  map[string]string
// @Router   /health [get].
func (s *State) Health(w http.ResponseWriter, r *http.Request) {
	s.JSON(w, http.StatusOK, map[string]string{
		statusJSONKey: "ok",
		"service":     courseServiceName,
		"version":     "1.0.0",
	})
}
