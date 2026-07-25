package model

// Garden is the aggregated result of GET /api/v1/garden ("Taman Luna") —
// entirely derived from daily_logs, no dedicated gamification table exists
// (see docs/FEATURE_REMINDERS_AND_GARDEN.md). Per PRD 4.5 / DESIGN.md's
// positive-only rule, this must never surface anything that measures
// absence — no missed-day counts, no breakable streaks.
type Garden struct {
	TotalLoggedDays     int
	LoggedDaysThisMonth int
	NewThisWeek         int
	CollectedMoods      []string
	UncollectedMoods    []string
	Message             string
}

type GardenResponse struct {
	TotalLoggedDays     int      `json:"total_logged_days"`
	LoggedDaysThisMonth int      `json:"logged_days_this_month"`
	NewThisWeek         int      `json:"new_this_week"`
	CollectedMoods      []string `json:"collected_moods"`
	UncollectedMoods    []string `json:"uncollected_moods"`
	Message             string   `json:"message"`
}

// ToResponse converts a domain Garden into its API response shape.
func (g Garden) ToResponse() GardenResponse {
	return GardenResponse{
		TotalLoggedDays:     g.TotalLoggedDays,
		LoggedDaysThisMonth: g.LoggedDaysThisMonth,
		NewThisWeek:         g.NewThisWeek,
		CollectedMoods:      g.CollectedMoods,
		UncollectedMoods:    g.UncollectedMoods,
		Message:             g.Message,
	}
}
