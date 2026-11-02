package content

import (
	"testing"
	"time"
)

// TestComputeCooldown_Fixed checks the fixed cooldown strategy.
func TestComputeCooldown_Fixed(t *testing.T) {
	t.Parallel()

	spec := CooldownSpec{Strategy: "fixed", BaseSeconds: 30}

	d := ComputeCooldown(1, spec)
	if d != 30*time.Second {
		t.Errorf("expected 30s, got %v", d)
	}

	d2 := ComputeCooldown(5, spec)
	if d2 != 30*time.Second {
		t.Errorf("fixed strategy should return constant 30s, got %v", d2)
	}
}

// TestComputeCooldown_Linear checks the linear cooldown strategy.
func TestComputeCooldown_Linear(t *testing.T) {
	t.Parallel()

	spec := CooldownSpec{Strategy: "linear", BaseSeconds: 10}

	d1 := ComputeCooldown(1, spec)
	if d1 != 10*time.Second {
		t.Errorf("expected 10s for attempt 1, got %v", d1)
	}

	d3 := ComputeCooldown(3, spec)
	if d3 != 30*time.Second {
		t.Errorf("expected 30s for attempt 3, got %v", d3)
	}
}

// TestComputeCooldown_Exponential checks the exponential strategy.
func TestComputeCooldown_Exponential(t *testing.T) {
	t.Parallel()

	spec := CooldownSpec{Strategy: "exponential", BaseSeconds: 10, Multiplier: 2.0}

	d1 := ComputeCooldown(1, spec)
	if d1 != 10*time.Second {
		t.Errorf("expected 10s for attempt 1, got %v", d1)
	}

	d2 := ComputeCooldown(2, spec)
	if d2 != 20*time.Second {
		t.Errorf("expected 20s for attempt 2, got %v", d2)
	}

	d3 := ComputeCooldown(3, spec)
	if d3 != 40*time.Second {
		t.Errorf("expected 40s for attempt 3, got %v", d3)
	}
}

// TestComputeCooldown_ExponentialMax checks the exponential cap.
func TestComputeCooldown_ExponentialMax(t *testing.T) {
	t.Parallel()

	spec := CooldownSpec{Strategy: "exponential", BaseSeconds: 10, Multiplier: 2.0, MaxSeconds: 25}

	d3 := ComputeCooldown(3, spec)
	if d3 != 25*time.Second {
		t.Errorf("expected capped at 25s, got %v", d3)
	}
}

// TestComputeCooldown_DefaultExponential checks the default strategy.
func TestComputeCooldown_DefaultExponential(t *testing.T) {
	t.Parallel()

	// Empty strategy defaults to exponential
	spec := CooldownSpec{BaseSeconds: 5, Multiplier: 3.0}

	d := ComputeCooldown(1, spec)
	if d != 5*time.Second {
		t.Errorf("expected 5s, got %v", d)
	}
}

// TestComputeCooldown_NeverNegative checks that a nonsensical negative
// configuration is clamped to zero rather than producing a deadline in the
// past, which would let a learner retry immediately.
func TestComputeCooldown_NeverNegative(t *testing.T) {
	t.Parallel()

	spec := CooldownSpec{Strategy: "fixed", BaseSeconds: -10}

	if d := ComputeCooldown(1, spec); d != 0 {
		t.Errorf("expected 0 for a negative base, got %v", d)
	}
}
