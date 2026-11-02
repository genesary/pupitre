package handlers

import (
	"context"
	"fmt"
	"net/http"

	"go.uber.org/zap"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// xpLevel thresholds and names for the five learner tiers.
const (
	xpLevelNovice    = 1
	xpLevelApprenant = 2
	xpLevelConfirme  = 3
	xpLevelExpert    = 4
	xpLevelMaitre    = 5

	xpThresholdApprenant = 200
	xpThresholdConfirme  = 600
	xpThresholdExpert    = 1500
	xpThresholdMaitre    = 3500
)

// levelNames maps a level number to its display name.
//
//nolint:gochecknoglobals // static lookup table, read-only after init
var levelNames = [xpLevelMaitre + 1]string{
	"",
	"Novice",
	"Apprenant",
	"Confirmé",
	"Expert",
	"Maître",
}

// xpLevel returns the tier (1–5) corresponding to total XP.
func xpLevel(total int) int {
	switch {
	case total >= xpThresholdMaitre:
		return xpLevelMaitre
	case total >= xpThresholdExpert:
		return xpLevelExpert
	case total >= xpThresholdConfirme:
		return xpLevelConfirme
	case total >= xpThresholdApprenant:
		return xpLevelApprenant
	default:
		return xpLevelNovice
	}
}

// xpResponse is the payload returned by MyXP.
type xpResponse struct {
	Total     int                     `json:"total"`
	Level     int                     `json:"level"`
	LevelName string                  `json:"levelName"`
	History   []repository.XPEventRow `json:"history"`
}

// MyXP godoc
// @Summary  Get the current user's XP total, level and history
// @Tags     XP
// @Security BearerAuth
// @Produce  json
// @Success  200  {object}  xpResponse
// @Router   /api/my/xp [get].
func (s *State) MyXP(writer http.ResponseWriter, req *http.Request) {
	claims := s.claims(req)

	history, err := s.Repos.XP.History(req.Context(), claims.Subject)
	if err != nil {
		zap.L().Error("failed to get xp history", zap.String("userID", claims.Subject), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Failed to fetch XP history")

		return
	}

	if history == nil {
		history = []repository.XPEventRow{}
	}

	total := 0
	for _, e := range history {
		total += e.Amount
	}

	lvl := xpLevel(total)
	s.JSON(writer, http.StatusOK, xpResponse{
		Total:     total,
		Level:     lvl,
		LevelName: levelNames[lvl],
		History:   history,
	})
}

// awardXP records an XP gain for userID, logging and returning any error.
// Callers that treat XP as non-critical may discard the return value.
func (s *State) awardXP(ctx context.Context, userID, source, sourceSlug string, amount int) error {
	err := s.Repos.XP.Award(ctx, userID, source, sourceSlug, amount)
	if err != nil {
		zap.L().Warn("failed to award xp",
			zap.String("userID", userID),
			zap.String("source", source),
			zap.String("sourceSlug", sourceSlug),
			zap.Error(err))

		return fmt.Errorf("award xp: %w", err)
	}

	return nil
}
