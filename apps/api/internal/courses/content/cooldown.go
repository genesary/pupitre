// Package content implements course content resolution, quiz scoring, and
// related domain logic for the course-service.
package content

import "time"

// Cooldown strategy identifiers understood by ComputeCooldown.
const (
	cooldownStrategyFixed  = "fixed"
	cooldownStrategyLinear = "linear"

	// CooldownStrategyExponential is the default backoff strategy, assumed
	// when a quiz module configures a cooldown without naming one.
	CooldownStrategyExponential = "exponential"
)

// ComputeCooldown calculates how long a learner must wait before retrying
// a question, given how many attempts they have now made and the module's
// configured backoff.
//
// The attempt counter itself is persisted (see models.QuizQuestionAttempt);
// this function stays pure so the backoff curve can be reasoned about and
// tested without a database.
func ComputeCooldown(currentAttempts int, spec CooldownSpec) time.Duration {
	var secs int

	switch spec.Strategy {
	case cooldownStrategyFixed:
		secs = spec.BaseSeconds
	case cooldownStrategyLinear:
		secs = spec.BaseSeconds * currentAttempts
	default: // exponential
		mult := 1.0
		for range currentAttempts - 1 {
			mult *= spec.Multiplier
		}

		secs = int(float64(spec.BaseSeconds) * mult)
		if spec.MaxSeconds > 0 && secs > spec.MaxSeconds {
			secs = spec.MaxSeconds
		}
	}

	if secs < 0 {
		secs = 0
	}

	return time.Duration(secs) * time.Second
}
