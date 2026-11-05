package handlers

import (
	"errors"
	"net/http"
	"testing"
)

// TestRequestError_Error returns the client-facing message verbatim and
// satisfies the error interface.
func TestRequestError_Error(t *testing.T) {
	t.Parallel()

	err := rejectf(http.StatusTeapot, "no %s allowed", "coffee")

	if err.Error() != "no coffee allowed" {
		t.Fatalf("Error() = %q, want the formatted message", err.Error())
	}

	var re requestError
	if !errors.As(err, &re) || re.status != http.StatusTeapot {
		t.Fatalf("rejectf did not carry the status through: %+v", err)
	}
}

// TestImportMode_Resolution accepts the three named modes, defaults a blank
// one to create, and rejects anything else with a 400.
func TestImportMode_Resolution(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", importModeCreate, false},
		{importModeCreate, importModeCreate, false},
		{importModeReplace, importModeReplace, false},
		{importModeAppend, importModeAppend, false},
		{"sideways", "", true},
	}

	for _, tc := range cases {
		got, err := importMode(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("importMode(%q): want error, got %q", tc.in, got)
			}

			var re requestError
			if err != nil && (!errors.As(err, &re) || re.status != http.StatusBadRequest) {
				t.Errorf("importMode(%q): want a 400 requestError, got %+v", tc.in, err)
			}

			continue
		}

		if err != nil || got != tc.want {
			t.Errorf("importMode(%q) = (%q, %v), want (%q, nil)", tc.in, got, err, tc.want)
		}
	}
}
