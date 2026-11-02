package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/jojo91550/pupitre2/apps/api/internal/platform/csvsafe"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// previewLimit is the maximum number of rows returned by ExportPreview.
const previewLimit = 10

// Virtual field ID constants used for cross-service enrichment detection.
const (
	exportFieldCourseTitle = "course_title"
	exportFieldPathTitle   = "path_title"
)

// exportRequest is the body accepted by ExportPreview and ExportDownload.
type exportRequest struct {
	Category string            `json:"category"`
	Fields   []string          `json:"fields"`
	Filters  map[string]string `json:"filters"`
}

// enrichSlugField maps each virtual export field ID to the SQL field that
// provides its lookup key for cross-service enrichment.
//
//nolint:gochecknoglobals // static read-only enrichment metadata
var enrichSlugField = map[string]string{
	exportFieldCourseTitle: "courseslug",
	exportFieldPathTitle:   "path_slug",
}

// titleEntry is a slug+title pair from course-service list responses.
type titleEntry struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

// ExportCategories returns the list of available export categories.
func (s *State) ExportCategories(writer http.ResponseWriter, req *http.Request) {
	cats := s.Repos.Export.Categories()
	s.JSON(writer, http.StatusOK, map[string]any{"categories": cats})
}

// ExportPreview returns the first 10 rows and the total count.
func (s *State) ExportPreview(writer http.ResponseWriter, req *http.Request) {
	var body exportRequest

	decodeErr := decode(req, &body)
	if decodeErr != nil {
		s.Error(writer, http.StatusBadRequest, "Corps de requête invalide")

		return
	}

	if body.Category == "" {
		s.Error(writer, http.StatusBadRequest, "Catégorie requise")

		return
	}

	sqlFields, virtualFields := splitVirtualFields(body.Fields)
	fetchFields, helperFields := buildFetchFields(sqlFields, virtualFields)

	scope, ok := s.exportScope(writer, req)
	if !ok {
		return
	}

	headers, rows, total, err := s.Repos.Export.FetchRows(req.Context(), body.Category, fetchFields, body.Filters, scope, previewLimit)
	if err != nil {
		if strings.HasPrefix(err.Error(), "unknown") {
			s.Error(writer, http.StatusBadRequest, err.Error())

			return
		}

		zap.L().Error("export preview failed", zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Erreur lors de la génération de l'aperçu")

		return
	}

	if len(virtualFields) > 0 {
		s.enrichRows(req.Context(), rows, virtualFields)
		rows = removeHelperFields(rows, helperFields)
		headers = body.Fields
	}

	s.JSON(writer, http.StatusOK, map[string]any{
		"headers":    headers,
		"rows":       rows,
		"totalCount": total,
	})
}

// exportScope is what the caller may export: everything for an admin, the
// members of their groups for a manager (docs/specs/190-backend-defects.md
// B6). It writes the error response and returns ok=false on failure.
func (s *State) exportScope(writer http.ResponseWriter, req *http.Request) (repository.ExportScope, bool) {
	claims := s.claims(req)
	if claims.Role != groupsRoleManager {
		return repository.Unrestricted, true
	}

	groups, err := s.Repos.Groups.GetGroupsByUserID(req.Context(), claims.Subject)
	if err != nil {
		zap.L().Error("load manager scope for export", zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return repository.ExportScope{}, false
	}

	return repository.ExportScope{Restricted: true, GroupIDs: managerScopeIDs(groups)}, true
}

// ExportDownload streams the full dataset as UTF-8 CSV and logs the download.
func (s *State) ExportDownload(writer http.ResponseWriter, req *http.Request) {
	claims := s.claims(req)

	var body exportRequest

	decodeErr := decode(req, &body)
	if decodeErr != nil {
		s.Error(writer, http.StatusBadRequest, "Corps de requête invalide")

		return
	}

	if body.Category == "" {
		s.Error(writer, http.StatusBadRequest, "Catégorie requise")

		return
	}

	scope, ok := s.exportScope(writer, req)
	if !ok {
		return
	}

	filename := fmt.Sprintf("%s-export-%s.csv", body.Category, time.Now().Format(time.DateOnly))

	writer.Header().Set("Content-Type", "text/csv; charset=utf-8")
	writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	_, _ = writer.Write([]byte{0xEF, 0xBB, 0xBF})

	sqlFields, virtualFields := splitVirtualFields(body.Fields)

	var rowCount int

	var writeErr error

	if len(virtualFields) == 0 {
		rowCount, writeErr = s.Repos.Export.WriteCSV(req.Context(), writer, body.Category, body.Fields, body.Filters, scope)
	} else {
		rowCount, writeErr = s.writeEnrichedCSV(req.Context(), writer, body.Category, body.Fields, sqlFields, virtualFields, body.Filters, scope)
	}

	if writeErr != nil {
		zap.L().Error("export download failed", zap.Error(writeErr))

		return
	}

	fieldsJSON, _ := json.Marshal(body.Fields) //nolint:errchkjson // marshalling []string never fails

	logEntry := &models.ExportLog{
		UserID:    claims.Subject,
		UserEmail: claims.Email,
		Category:  body.Category,
		Fields:    string(fieldsJSON),
		RowCount:  rowCount,
	}

	logErr := s.Repos.Export.LogDownload(req.Context(), logEntry)
	if logErr != nil {
		zap.L().Warn("export audit log failed", zap.Error(logErr))
	}
}

// writeEnrichedCSV streams the dataset as CSV, filling in virtual fields
// as each row goes past. Called only when at least one virtual field is in
// the requested fields.
//
// The slug→title maps the enrichment needs are fetched once up front, so
// the rows themselves never have to be held: this used to load the entire
// result set into memory before writing a single byte, which put the peak
// memory of an export at the size of the export.
func (s *State) writeEnrichedCSV(
	ctx context.Context,
	writer io.Writer,
	category string,
	origFields []string,
	sqlFields []string,
	virtualFields []string,
	filters map[string]string,
	scope repository.ExportScope,
) (int, error) {
	fetchFields, _ := buildFetchFields(sqlFields, virtualFields)
	enrich := s.rowEnricher(ctx, virtualFields)

	csvWriter := csv.NewWriter(writer)
	csvWriter.Comma = ';'

	headerErr := csvWriter.Write(append([]string(nil), origFields...))
	if headerErr != nil {
		return 0, fmt.Errorf("write csv header: %w", headerErr)
	}

	rowCount := 0
	record := make([]string, len(origFields))

	_, streamErr := s.Repos.Export.StreamRows(ctx, category, fetchFields, filters, scope,
		func(rowData map[string]string) error {
			enrich(rowData)

			for idx, fieldID := range origFields {
				record[idx] = rowData[fieldID]
			}

			rowErr := csvWriter.Write(csvsafe.Record(record))
			if rowErr != nil {
				return fmt.Errorf("write csv row: %w", rowErr)
			}

			rowCount++

			return nil
		})
	if streamErr != nil {
		return rowCount, fmt.Errorf("stream rows: %w", streamErr)
	}

	csvWriter.Flush()

	flushErr := csvWriter.Error()
	if flushErr != nil {
		return rowCount, fmt.Errorf("csv flush: %w", flushErr)
	}

	return rowCount, nil
}

// splitVirtualFields partitions fields into SQL fields (returned by the DB)
// and virtual fields (populated by cross-service enrichment).
//
//nolint:gocritic // unnamedResult conflicts with project nonamedreturns policy
func splitVirtualFields(fields []string) ([]string, []string) {
	var sqlFields, virtualFields []string

	for _, fieldID := range fields {
		if repository.IsVirtualExportField(fieldID) {
			virtualFields = append(virtualFields, fieldID)
		} else {
			sqlFields = append(sqlFields, fieldID)
		}
	}

	return sqlFields, virtualFields
}

// buildFetchFields returns the fields to pass to FetchRows: sqlFields plus
// any slug fields for virtual enrichment not already in sqlFields.
// helperFields lists extra slug fields added for enrichment purposes.
//
//nolint:gocritic // unnamedResult conflicts with project nonamedreturns policy
func buildFetchFields(sqlFields, virtualFields []string) ([]string, map[string]bool) {
	inSQL := make(map[string]bool, len(sqlFields))

	for _, fieldID := range sqlFields {
		inSQL[fieldID] = true
	}

	helperFields := make(map[string]bool)

	var extra []string

	for _, virtualField := range virtualFields {
		slugField, found := enrichSlugField[virtualField]
		if !found || inSQL[slugField] {
			continue
		}

		if !helperFields[slugField] {
			helperFields[slugField] = true
			extra = append(extra, slugField)
		}
	}

	fetchFields := make([]string, 0, len(sqlFields)+len(extra))
	fetchFields = append(fetchFields, sqlFields...)
	fetchFields = append(fetchFields, extra...)

	return fetchFields, helperFields
}

// removeHelperFields removes from each row the fields that were added as
// enrichment helpers but were not in the original field selection.
func removeHelperFields(rows []map[string]string, helperFields map[string]bool) []map[string]string {
	if len(helperFields) == 0 {
		return rows
	}

	// Collect keys first so neither the rows slice nor helperFields is
	// modified while being iterated.
	toDelete := make([]string, 0, len(helperFields))
	for fieldID := range helperFields {
		toDelete = append(toDelete, fieldID)
	}

	for _, rowData := range rows {
		for _, fieldID := range toDelete {
			delete(rowData, fieldID)
		}
	}

	return rows
}

// rowEnricher fetches the slug→title maps the requested virtual fields
// need — once — and returns a function that fills them into a single row.
//
// Splitting the lookup from the per-row work is what lets both the preview
// (a slice of rows) and the download (a stream) share one implementation
// without either paying for the lookup more than once.
func (s *State) rowEnricher(ctx context.Context, virtualFields []string) func(row map[string]string) {
	needCourse := false
	needPath := false

	for _, fieldID := range virtualFields {
		if fieldID == exportFieldCourseTitle {
			needCourse = true
		}

		if fieldID == exportFieldPathTitle {
			needPath = true
		}
	}

	var courseTitles map[string]string
	if needCourse {
		courseTitles, _ = s.fetchCourseTitles(ctx)
	}

	var pathTitles map[string]string
	if needPath {
		pathTitles, _ = s.fetchPathTitles(ctx)
	}

	return func(row map[string]string) {
		if needCourse {
			row[exportFieldCourseTitle] = courseTitles[row["courseslug"]]
		}

		if needPath {
			row[exportFieldPathTitle] = pathTitles[row["path_slug"]]
		}
	}
}

// enrichRows fills in virtual field values for every row in a preview.
func (s *State) enrichRows(ctx context.Context, rows []map[string]string, virtualFields []string) {
	enrich := s.rowEnricher(ctx, virtualFields)
	for _, row := range rows {
		enrich(row)
	}
}

// fetchCourseTitles returns the slug→title map of the public courses.
func (s *State) fetchCourseTitles(ctx context.Context) (map[string]string, error) {
	return s.courseSource().PublicCourseTitles(ctx) //nolint:wrapcheck // same-process call, already wrapped
}

// fetchPathTitles returns the slug→title map of the paths.
func (s *State) fetchPathTitles(ctx context.Context) (map[string]string, error) {
	return s.courseSource().PathTitles(ctx) //nolint:wrapcheck // same-process call, already wrapped
}
