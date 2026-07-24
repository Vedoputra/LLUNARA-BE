package model

import (
	"time"

	"github.com/google/uuid"
)

// WellnessLog is the domain representation of a day's wellness metrics.
// Every field is optional — users fill in only what they choose to track.
type WellnessLog struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Date         time.Time
	WaterGlasses *int
	SleepHours   *float64
	WeightKg     *float64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// UpsertWellnessLogRequest is the payload for POST /api/v1/wellness.
type UpsertWellnessLogRequest struct {
	Date         string   `json:"date" validate:"required,datetime=2006-01-02"`
	WaterGlasses *int     `json:"water_glasses,omitempty" validate:"omitempty,min=0,max=30"`
	SleepHours   *float64 `json:"sleep_hours,omitempty" validate:"omitempty,min=0,max=24"`
	WeightKg     *float64 `json:"weight_kg,omitempty" validate:"omitempty,min=20,max=300"`
}

// ParseDate parses Date using DateLayout.
func (r UpsertWellnessLogRequest) ParseDate() (time.Time, error) {
	return time.Parse(DateLayout, r.Date)
}

// WellnessLogResponse is the JSON representation of a WellnessLog.
type WellnessLogResponse struct {
	ID           string   `json:"id"`
	Date         string   `json:"date"`
	WaterGlasses *int     `json:"water_glasses,omitempty"`
	SleepHours   *float64 `json:"sleep_hours,omitempty"`
	WeightKg     *float64 `json:"weight_kg,omitempty"`
}

// ToResponse converts a domain WellnessLog into its API response shape.
func (w WellnessLog) ToResponse() WellnessLogResponse {
	return WellnessLogResponse{
		ID:           w.ID.String(),
		Date:         w.Date.Format(DateLayout),
		WaterGlasses: w.WaterGlasses,
		SleepHours:   w.SleepHours,
		WeightKg:     w.WeightKg,
	}
}
