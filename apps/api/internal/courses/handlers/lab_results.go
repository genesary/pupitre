package handlers

import (
	"net/http"
	"time"
)

// labCheckRow is a single row of the admin lab-checks report, returned
// as-is to the frontend admin dashboard.
type labCheckRow struct {
	ID          int64     `json:"id"`
	Username    string    `json:"username"`
	CourseSlug  string    `json:"courseSlug"`
	ModuleIndex int       `json:"moduleIndex"`
	ModuleName  string    `json:"moduleName"`
	Allow       bool      `json:"allow"`
	Violations  []string  `json:"violations"`
	Verified    bool      `json:"verified"`
	CheckedAt   time.Time `json:"checkedAt"`
}

// GetLabResults godoc
// @Summary   List recorded lab checks (admin)
// @Tags      Admin
// @Security  BearerAuth
// @Produce   json
// @Param     course  query  string  false  "Filter by course slug"
// @Success   200  {array}  labCheckRow
// @Failure   503  {object}  map[string]string
// @Router    /api/admin/lab-checks [get].
func (s *State) GetLabResults(writer http.ResponseWriter, req *http.Request) {
	courseSlug := req.URL.Query().Get("course")

	checks, err := s.Repos.LabChecks.List(req.Context(), courseSlug)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "db query failed")

		return
	}

	results := make([]labCheckRow, 0, len(checks))
	for _, check := range checks {
		results = append(results, labCheckRow{
			ID:          check.ID,
			Username:    check.Username,
			CourseSlug:  check.CourseSlug,
			ModuleIndex: check.ModuleIndex,
			ModuleName:  check.ModuleName,
			Allow:       check.Allow,
			Violations:  []string(check.Violations),
			Verified:    check.Verified,
			CheckedAt:   check.CheckedAt,
		})
	}

	s.JSON(writer, http.StatusOK, results)
}
