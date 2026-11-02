package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/models"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/repository"
	"github.com/jojo91550/pupitre2/apps/api/internal/platform/csvsafe"
)

// exportPreviewLimit is the max rows returned by ExportLabPreview.
const exportPreviewLimit = 10

// Lab check export field ID constants.
const (
	labFieldID          = "id"
	labFieldUsername    = "username"
	labFieldCourseSlug  = "courseslug"
	labFieldModuleIndex = "moduleindex"
	labFieldModuleName  = "modulename"
	labFieldAllow       = "allow"
	labFieldViolations  = "violations"
	labFieldVerified    = "verified"
	labFieldCheckedAt   = "checkedat"
)

// Lab check export display constants.
const (
	labAllowLabelPass = "Réussi"
	labAllowLabelFail = "Échoué"
	labResultLabel    = "Résultat"
	filterTypeDate    = "date"
)

// labFieldLabel maps field IDs to their display labels.
//
//nolint:gochecknoglobals // static read-only export schema
var labFieldLabel = map[string]string{
	labFieldID:          "ID",
	labFieldUsername:    "Nom d'utilisateur",
	labFieldCourseSlug:  "Slug cours",
	labFieldModuleIndex: "Index module",
	labFieldModuleName:  "Nom module",
	labFieldAllow:       labResultLabel,
	labFieldViolations:  "Violations",
	labFieldVerified:    "Vérifié",
	labFieldCheckedAt:   "Vérifié le",
}

// labFieldOrder is the canonical display order for lab check fields.
//
//nolint:gochecknoglobals // static read-only export schema
var labFieldOrder = []string{
	labFieldID,
	labFieldUsername,
	labFieldCourseSlug,
	labFieldModuleIndex,
	labFieldModuleName,
	labFieldAllow,
	labFieldViolations,
	labFieldVerified,
	labFieldCheckedAt,
}

// labDefaultFields is the default field selection shown in the export UI.
//
//nolint:gochecknoglobals // static read-only export schema
var labDefaultFields = []string{
	labFieldUsername,
	labFieldCourseSlug,
	labFieldModuleName,
	labFieldAllow,
	labFieldCheckedAt,
}

// labExportFieldMeta is the JSON payload for one exportable field.
type labExportFieldMeta struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// labExportFilterOpt is a (value, label) pair for select-type filters.
type labExportFilterOpt struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// labExportFilterMeta is the JSON payload for one available filter.
type labExportFilterMeta struct {
	ID      string               `json:"id"`
	Label   string               `json:"label"`
	Type    string               `json:"type"`
	Options []labExportFilterOpt `json:"options,omitempty"`
}

// labExportCategoryMeta is the JSON payload returned by ExportLabCategories.
type labExportCategoryMeta struct {
	ID            string                `json:"id"`
	Label         string                `json:"label"`
	Fields        []labExportFieldMeta  `json:"fields"`
	DefaultFields []string              `json:"defaultFields"`
	Filters       []labExportFilterMeta `json:"filters"`
}

// labExportRequest is the body accepted by the lab export handlers.
type labExportRequest struct {
	Fields  []string          `json:"fields"`
	Filters map[string]string `json:"filters"`
}

// ExportLabCategories returns the lab_checks category description.
func (s *State) ExportLabCategories(writer http.ResponseWriter, req *http.Request) {
	fields := make([]labExportFieldMeta, 0, len(labFieldOrder))
	for _, fieldID := range labFieldOrder {
		fields = append(fields, labExportFieldMeta{ID: fieldID, Label: labFieldLabel[fieldID]})
	}

	cat := labExportCategoryMeta{
		ID:            "lab_checks",
		Label:         "Lab Checks",
		Fields:        fields,
		DefaultFields: labDefaultFields,
		Filters: []labExportFilterMeta{
			{ID: "course_slug", Label: "Cours (slug)", Type: "text"},
			{ID: "checked_from", Label: "Vérifié depuis", Type: filterTypeDate},
			{ID: "checked_to", Label: "Vérifié jusqu'au", Type: filterTypeDate},
			{
				ID: "allow", Label: labResultLabel, Type: "select",
				Options: []labExportFilterOpt{
					{Value: "true", Label: labAllowLabelPass},
					{Value: "false", Label: labAllowLabelFail},
				},
			},
		},
	}

	s.JSON(writer, http.StatusOK, map[string]any{"categories": []labExportCategoryMeta{cat}})
}

// ExportLabPreview returns the first 10 lab check rows matching the filters.
func (s *State) ExportLabPreview(writer http.ResponseWriter, req *http.Request) {
	var body labExportRequest

	decodeErr := decode(req, &body)
	if decodeErr != nil {
		s.Error(writer, http.StatusBadRequest, "Corps de requête invalide")

		return
	}

	fields := resolvedFields(body.Fields)
	labFilter := parseLabFilter(body.Filters)

	rows, total, err := s.Repos.LabChecks.ListExport(req.Context(), labFilter, exportPreviewLimit)
	if err != nil {
		zap.L().Error("lab export preview failed", zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Erreur lors de la génération de l'aperçu")

		return
	}

	jsonRows := make([]map[string]string, 0, len(rows))
	for rowIdx := range rows {
		jsonRows = append(jsonRows, labCheckToRow(&rows[rowIdx], fields))
	}

	s.JSON(writer, http.StatusOK, map[string]any{
		"headers":    fields,
		"rows":       jsonRows,
		"totalCount": total,
	})
}

// ExportLabDownload streams all lab check rows matching the filters as CSV.
// Rows are written to the response one at a time — the full result set is
// never loaded into memory.
func (s *State) ExportLabDownload(writer http.ResponseWriter, req *http.Request) {
	var body labExportRequest

	decodeErr := decode(req, &body)
	if decodeErr != nil {
		s.Error(writer, http.StatusBadRequest, "Corps de requête invalide")

		return
	}

	fields := resolvedFields(body.Fields)
	labFilter := parseLabFilter(body.Filters)

	filename := fmt.Sprintf("lab_checks-export-%s.csv", time.Now().Format(time.DateOnly))

	writer.Header().Set("Content-Type", "text/csv; charset=utf-8")
	writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	_, _ = writer.Write([]byte{0xEF, 0xBB, 0xBF})

	csvWriter := csv.NewWriter(writer)
	csvWriter.Comma = ';'

	defer csvWriter.Flush()

	headers := make([]string, 0, len(fields))
	for _, fieldID := range fields {
		if lbl, ok := labFieldLabel[fieldID]; ok {
			headers = append(headers, lbl)
		} else {
			headers = append(headers, fieldID)
		}
	}

	headerErr := csvWriter.Write(headers)
	if headerErr != nil {
		zap.L().Error("lab export write header", zap.Error(headerErr))

		return
	}

	streamErr := s.Repos.LabChecks.StreamExport(req.Context(), labFilter, func(row *models.LabCheck) error {
		return csvWriter.Write(csvsafe.Record(labCheckToRecord(row, fields)))
	})
	if streamErr != nil {
		zap.L().Error("lab export download failed", zap.Error(streamErr))
	}
}

// resolvedFields returns fields if non-empty, or the default field list.
func resolvedFields(fields []string) []string {
	if len(fields) > 0 {
		return fields
	}

	return labDefaultFields
}

// labCheckToRow converts one LabCheck to a map for the requested fields
// (used for JSON preview).
func labCheckToRow(labCheck *models.LabCheck, fields []string) map[string]string {
	all := labCheckAllFields(labCheck)
	out := make(map[string]string, len(fields))

	for _, fieldID := range fields {
		out[fieldID] = all[fieldID]
	}

	return out
}

// labCheckToRecord converts one LabCheck to an ordered []string for CSV.
func labCheckToRecord(labCheck *models.LabCheck, fields []string) []string {
	all := labCheckAllFields(labCheck)
	record := make([]string, 0, len(fields))

	for _, fieldID := range fields {
		record = append(record, all[fieldID])
	}

	return record
}

// labCheckAllFields returns all field values for a LabCheck as a string map.
func labCheckAllFields(labCheck *models.LabCheck) map[string]string {
	allowLabel := labAllowLabelFail
	if labCheck.Allow {
		allowLabel = labAllowLabelPass
	}

	return map[string]string{
		labFieldID:          strconv.FormatInt(labCheck.ID, 10),
		labFieldUsername:    labCheck.Username,
		labFieldCourseSlug:  labCheck.CourseSlug,
		labFieldModuleIndex: strconv.Itoa(labCheck.ModuleIndex),
		labFieldModuleName:  labCheck.ModuleName,
		labFieldAllow:       allowLabel,
		labFieldViolations:  strings.Join(labCheck.Violations, "; "),
		labFieldVerified:    strconv.FormatBool(labCheck.Verified),
		labFieldCheckedAt:   labCheck.CheckedAt.Format(time.RFC3339),
	}
}

// parseLabFilter converts the request filter map to a typed LabCheckFilter.
func parseLabFilter(filters map[string]string) repository.LabCheckFilter {
	labFilter := repository.LabCheckFilter{}

	if courseSlug := filters["course_slug"]; courseSlug != "" {
		labFilter.CourseSlug = courseSlug
	}

	if allowStr := filters["allow"]; allowStr != "" {
		parsed, err := strconv.ParseBool(allowStr)
		if err == nil {
			labFilter.Allow = &parsed
		}
	}

	if fromStr := filters["checked_from"]; fromStr != "" {
		parsed, err := time.Parse(time.DateOnly, fromStr)
		if err == nil {
			labFilter.From = &parsed
		}
	}

	if toStr := filters["checked_to"]; toStr != "" {
		parsed, err := time.Parse(time.DateOnly, toStr)
		if err == nil {
			labFilter.To = &parsed
		}
	}

	return labFilter
}
