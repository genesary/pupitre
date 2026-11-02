package csvsafe

import "testing"

// TestCell is the regression test of defect B8.
func TestCell(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"":                        "",
		"Alice":                   "Alice",
		"alice@example.com":       "alice@example.com",
		"2026-10-26":              "2026-10-26",
		"=HYPERLINK(\"x\",\"y\")": "'=HYPERLINK(\"x\",\"y\")",
		"+33 6":                   "'+33 6",
		"-2+3":                    "'-2+3",
		"@SUM(A1)":                "'@SUM(A1)",
		"\t=1":                    "'\t=1",
		"\r=1":                    "'\r=1",
	}

	for in, want := range cases {
		if got := Cell(in); got != want {
			t.Errorf("Cell(%q) = %q, want %q", in, got, want)
		}
	}
}
