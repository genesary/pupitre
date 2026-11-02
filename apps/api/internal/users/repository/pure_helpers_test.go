package repository

import "testing"

// TestDifficultyOrder maps each known difficulty to its rank and everything
// else to zero.
func TestDifficultyOrder(t *testing.T) {
	t.Parallel()

	cases := map[string]int{
		"beginner":     difficultyOrderBeginner,
		"intermediate": difficultyOrderIntermediate,
		"advanced":     difficultyOrderAdvanced,
		"maître":       0,
		"":             0,
		"unknown":      0,
	}

	for in, want := range cases {
		if got := difficultyOrder(in); got != want {
			t.Errorf("difficultyOrder(%q) = %d, want %d", in, got, want)
		}
	}

	if difficultyOrderBeginner >= difficultyOrderIntermediate ||
		difficultyOrderIntermediate >= difficultyOrderAdvanced {
		t.Error("difficulty ranks are not strictly increasing")
	}
}
