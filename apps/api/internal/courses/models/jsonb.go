package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"reflect"
)

// JSONB is a raw JSON document stored in a Postgres `jsonb` column.
//
// It deliberately stays a byte slice rather than a decoded Go value: the
// repository layer owns the mapping between these documents and the
// domain types in internal/content, which keeps this package free of any
// dependency on them.
//
// The mixed receivers below are required by the interfaces it implements:
// [sql.Scanner] must take a pointer to overwrite the value, while
// [driver.Valuer] and GORM's data-type hook are called on the field value.
//
//nolint:recvcheck // Scanner needs a pointer receiver, Valuer a value one
type JSONB []byte

// Value implements [driver.Valuer], writing NULL for an empty document so
// that "no value" and "the JSON literal null" stay distinguishable.
func (j JSONB) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil //nolint:nilnil // a NULL column is exactly what an empty document maps to
	}

	return []byte(j), nil
}

// Scan implements [sql.Scanner] for the []byte and string forms pgx
// returns for jsonb columns.
func (j *JSONB) Scan(src any) error {
	switch typed := src.(type) {
	case nil:
		*j = nil
	case []byte:
		*j = append(JSONB(nil), typed...)
	case string:
		*j = JSONB(typed)
	default:
		return fmt.Errorf("scan jsonb: unsupported source type %T", src)
	}

	return nil
}

// GormDataType pins the column type so AutoMigrate creates jsonb rather
// than bytea.
func (JSONB) GormDataType() string {
	return "jsonb"
}

// MarshalJSONB encodes value into a JSONB document, returning nil (a SQL
// NULL) for nil or empty slices/maps so absent values do not round-trip as
// the literal "null" or "[]".
func MarshalJSONB(value any) (JSONB, error) {
	if isEmptyDocument(value) {
		return nil, nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal jsonb document: %w", err)
	}

	return data, nil
}

// UnmarshalJSONB decodes doc into dst, treating an empty document as a
// no-op so dst keeps its zero value.
func UnmarshalJSONB(doc JSONB, dst any) error {
	if len(doc) == 0 {
		return nil
	}

	err := json.Unmarshal(doc, dst)
	if err != nil {
		return fmt.Errorf("unmarshal jsonb document: %w", err)
	}

	return nil
}

// isEmptyDocument reports whether value carries nothing worth persisting:
// a nil interface, a nil pointer, or an empty slice or map. Every other
// kind — including a zero-valued struct or number — is persisted as-is.
func isEmptyDocument(value any) bool {
	if value == nil {
		return true
	}

	reflected := reflect.ValueOf(value)

	switch reflected.Kind() { //nolint:exhaustive // only container kinds can be empty; the default covers the rest
	case reflect.Slice, reflect.Map:
		return reflected.Len() == 0
	case reflect.Pointer, reflect.Interface:
		return reflected.IsNil()
	default:
		return false
	}
}
