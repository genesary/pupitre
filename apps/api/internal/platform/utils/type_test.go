package utils

import (
	"reflect"
	"testing"
)

// TestSplitTrimmedCommaList verifies comma splitting, whitespace trimming,
// and empty-entry removal.
func TestSplitTrimmedCommaList(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input string
		want  []string
	}{
		"empty":       {input: "", want: nil},
		"only commas": {input: ",,,", want: nil},
		"single":      {input: "admins", want: []string{"admins"}},
		"multiple":    {input: "admins,seanergy-admins, ops ", want: []string{"admins", "seanergy-admins", "ops"}},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := SplitTrimmedCommaList(test.input); !reflect.DeepEqual(got, test.want) {
				t.Errorf("SplitTrimmedCommaList(%q) = %v, want %v", test.input, got, test.want)
			}
		})
	}
}
