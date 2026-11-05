package models

import (
	"bytes"
	"testing"
)

// TestJSONB_Value maps an empty document to a SQL NULL and a populated one
// to its bytes.
func TestJSONB_Value(t *testing.T) {
	t.Parallel()

	v, err := JSONB(nil).Value()
	if err != nil || v != nil {
		t.Errorf("empty JSONB Value() = %v, %v; want nil, nil", v, err)
	}

	v, err = JSONB(`{"a":1}`).Value()
	if err != nil {
		t.Fatalf("Value(): %v", err)
	}

	b, ok := v.([]byte)
	if !ok || string(b) != `{"a":1}` {
		t.Errorf("Value() = %v (%T), want []byte(`{\"a\":1}`)", v, v)
	}
}

// TestJSONB_Scan accepts nil, []byte and string, and rejects other types.
func TestJSONB_Scan(t *testing.T) {
	t.Parallel()

	var j JSONB

	err := j.Scan(nil)
	if err != nil || j != nil {
		t.Errorf("Scan(nil) = %v, j=%v", err, j)
	}

	src := []byte(`{"x":true}`)

	err = j.Scan(src)
	if err != nil {
		t.Fatalf("Scan([]byte): %v", err)
	}

	if !bytes.Equal(j, src) {
		t.Errorf("Scan([]byte) stored %s", j)
	}

	// The copy must be independent of the source slice.
	src[0] = 'X'

	if j[0] == 'X' {
		t.Error("Scan([]byte) did not copy the source slice")
	}

	err = j.Scan("literal")
	if err != nil || string(j) != "literal" {
		t.Errorf("Scan(string) = %v, j=%s", err, j)
	}

	err = j.Scan(42)
	if err == nil {
		t.Error("Scan(int) should return an error")
	}
}

// TestJSONB_GormDataType pins the column type.
func TestJSONB_GormDataType(t *testing.T) {
	t.Parallel()

	if got := JSONB(nil).GormDataType(); got != "jsonb" {
		t.Errorf("GormDataType() = %q", got)
	}
}

// TestMarshalJSONB returns NULL for empty documents and JSON otherwise.
func TestMarshalJSONB(t *testing.T) {
	t.Parallel()

	empties := []any{nil, []string(nil), []int{}, map[string]int{}, (*int)(nil)}
	for _, e := range empties {
		got, err := MarshalJSONB(e)
		if err != nil || got != nil {
			t.Errorf("MarshalJSONB(%#v) = %v, %v; want nil, nil", e, got, err)
		}
	}

	got, err := MarshalJSONB(map[string]int{"n": 1})
	if err != nil || string(got) != `{"n":1}` {
		t.Errorf("MarshalJSONB(map) = %s, %v", got, err)
	}

	// A zero-valued struct is still worth persisting.
	got, err = MarshalJSONB(struct {
		A int `json:"a"`
	}{})
	if err != nil || string(got) != `{"a":0}` {
		t.Errorf("MarshalJSONB(zero struct) = %s, %v", got, err)
	}

	_, err = MarshalJSONB(make(chan int))
	if err == nil {
		t.Error("MarshalJSONB(chan) should fail")
	}
}

// TestUnmarshalJSONB is a no-op for empty docs and decodes otherwise.
func TestUnmarshalJSONB(t *testing.T) {
	t.Parallel()

	var dst struct {
		N int `json:"n"`
	}

	dst.N = 7

	err := UnmarshalJSONB(nil, &dst)
	if err != nil || dst.N != 7 {
		t.Errorf("empty doc should leave dst untouched: err=%v N=%d", err, dst.N)
	}

	err = UnmarshalJSONB(JSONB(`{"n":3}`), &dst)
	if err != nil || dst.N != 3 {
		t.Errorf("UnmarshalJSONB = %v, N=%d", err, dst.N)
	}

	err = UnmarshalJSONB(JSONB(`{bad json`), &dst)
	if err == nil {
		t.Error("UnmarshalJSONB(bad) should fail")
	}
}

// TestTableNames pins every model's mapped table name.
func TestTableNames(t *testing.T) {
	t.Parallel()

	type namer interface{ TableName() string }

	cases := []struct {
		model namer
		want  string
	}{
		{QuizQuestionAttempt{}, "quiz_question_attempts"},
		{Path{}, "paths"},
		{PathCourse{}, "path_courses"},
		{PathSkill{}, "path_skills"},
		{LabCheck{}, "lab_checks"},
		{Course{}, "courses"},
		{CourseModule{}, "course_modules"},
		{CoursePrerequisite{}, "course_prerequisites"},
		{CourseSession{}, "course_sessions"},
	}

	for _, tc := range cases {
		if got := tc.model.TableName(); got != tc.want {
			t.Errorf("%T.TableName() = %q, want %q", tc.model, got, tc.want)
		}
	}
}
