package fake

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"maps"
	"sync"

	"github.com/jojo91550/pupitre2/apps/api/internal/platform/csvsafe"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// exportFieldEmail is the demo field ID used by the default fake category.
const exportFieldEmail = "email"

// ExportRepository is an in-memory repository.ExportRepository for tests.
type ExportRepository struct {
	mu sync.Mutex

	// Categories returned by the Categories method.
	Cats []repository.ExportCategoryMeta

	// Headers and Rows are returned verbatim by FetchRows (Rows is also the
	// source for WriteCSV). Total defaults to len(Rows) when zero.
	Headers []string
	Rows    []map[string]string
	Total   int64

	// FetchErr, when set, is returned by FetchRows and WriteCSV. Use an
	// error whose message starts with "unknown" to exercise the handler's
	// 400 path.
	FetchErr error
	// LogErr, when set, is returned by LogDownload.
	LogErr error

	// Logged captures every ExportLog passed to LogDownload.
	Logged []models.ExportLog

	// LastScope is the scope of the last FetchRows, WriteCSV or StreamRows.
	LastScope repository.ExportScope
}

// NewExportRepository builds a fake ExportRepository with one demo category.
func NewExportRepository() *ExportRepository {
	return &ExportRepository{
		Cats: []repository.ExportCategoryMeta{
			{
				ID:            "users",
				Label:         "Users",
				Fields:        []repository.ExportFieldMeta{{ID: exportFieldEmail, Label: "Email"}},
				DefaultFields: []string{exportFieldEmail},
			},
		},
	}
}

// Categories returns the configured category metadata.
func (f *ExportRepository) Categories() []repository.ExportCategoryMeta {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.Cats
}

// FetchRows returns the configured headers, rows and total for any category.
//
//nolint:gocritic // result shape is fixed by the ExportRepository interface
func (f *ExportRepository) FetchRows(
	_ context.Context, _ string, fields []string, _ map[string]string, scope repository.ExportScope, limit int,
) ([]string, []map[string]string, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.LastScope = scope

	if f.FetchErr != nil {
		return nil, nil, 0, f.FetchErr
	}

	headers := f.Headers
	if headers == nil {
		headers = append([]string(nil), fields...)
	}

	rows := f.cloneRows()
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}

	total := f.Total
	if total == 0 {
		total = int64(len(f.Rows))
	}

	return headers, rows, total, nil
}

// WriteCSV streams the configured rows into writer as semicolon-delimited CSV.
func (f *ExportRepository) WriteCSV(
	_ context.Context, writer io.Writer, _ string, fields []string, _ map[string]string, scope repository.ExportScope,
) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.LastScope = scope

	if f.FetchErr != nil {
		return 0, f.FetchErr
	}

	csvWriter := csv.NewWriter(writer)
	csvWriter.Comma = ';'

	err := csvWriter.Write(append([]string(nil), fields...))
	if err != nil {
		return 0, fmt.Errorf("write header: %w", err)
	}

	for _, row := range f.Rows {
		record := make([]string, 0, len(fields))
		for _, field := range fields {
			record = append(record, row[field])
		}

		err = csvWriter.Write(csvsafe.Record(record))
		if err != nil {
			return 0, fmt.Errorf("write row: %w", err)
		}
	}

	csvWriter.Flush()

	err = csvWriter.Error()
	if err != nil {
		return 0, fmt.Errorf("flush: %w", err)
	}

	return len(f.Rows), nil
}

// LogDownload records the audit entry, or returns the configured LogErr.
func (f *ExportRepository) LogDownload(_ context.Context, log *models.ExportLog) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.LogErr != nil {
		return f.LogErr
	}

	if log == nil {
		return errors.New("nil export log")
	}

	f.Logged = append(f.Logged, *log)

	return nil
}

// StreamRows hands each configured row to visit, returning the headers.
func (f *ExportRepository) StreamRows(
	_ context.Context, _ string, fields []string, _ map[string]string, scope repository.ExportScope,
	visit func(map[string]string) error,
) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.LastScope = scope

	if f.FetchErr != nil {
		return nil, f.FetchErr
	}

	headers := f.Headers
	if headers == nil {
		headers = append([]string(nil), fields...)
	}

	for _, row := range f.cloneRows() {
		err := visit(row)
		if err != nil {
			return nil, err
		}
	}

	return headers, nil
}

// cloneRows returns a deep copy of f.Rows so callers cannot mutate fixtures.
func (f *ExportRepository) cloneRows() []map[string]string {
	out := make([]map[string]string, len(f.Rows))

	for idx, row := range f.Rows {
		clone := make(map[string]string, len(row))
		maps.Copy(clone, row)
		out[idx] = clone
	}

	return out
}
