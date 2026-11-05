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

// TestDeriveRole verifies the strongest mapped role wins, in any group order.
func TestDeriveRole(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		current string
		mapped  []string
		want    string
	}{
		{"no mapping keeps the role", roleManager, nil, roleManager},
		{"a mapping applies", roleStudent, []string{roleManager}, roleManager},
		{"manager then student", roleStudent, []string{roleManager, roleStudent}, roleManager},
		{"student then manager", roleStudent, []string{roleStudent, roleManager}, roleManager},
		{"admin beats manager", roleStudent, []string{roleAdmin, roleManager}, roleAdmin},
		{"an admin stays one", roleAdmin, []string{roleStudent}, roleAdmin},
		{"a mapping can demote a manager", roleManager, []string{roleStudent}, roleStudent},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := DeriveRole(tc.current, tc.mapped); got != tc.want {
				t.Errorf("DeriveRole(%q, %v) = %q, want %q", tc.current, tc.mapped, got, tc.want)
			}
		})
	}
}
