package model

import (
	"time"

	"github.com/google/uuid"
)

// DateLayout is the canonical date-only format used across API request and
// response payloads (e.g. "2026-08-12").
const DateLayout = "2006-01-02"

// Cycle is the domain representation of a menstrual cycle.
type Cycle struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	StartDate    time.Time
	EndDate      *time.Time
	CycleLength  *int
	PeriodLength *int
	IsOutlier    bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// StartCycleRequest is the payload for POST /api/v1/cycles.
type StartCycleRequest struct {
	StartDate string `json:"start_date" validate:"required,datetime=2006-01-02"`
}

// ParseStartDate parses StartDate using DateLayout.
func (r StartCycleRequest) ParseStartDate() (time.Time, error) {
	return time.Parse(DateLayout, r.StartDate)
}

// UpdateCycleRequest is the payload for PATCH /api/v1/cycles/{id}.
type UpdateCycleRequest struct {
	EndDate string `json:"end_date" validate:"required,datetime=2006-01-02"`
}

// ParseEndDate parses EndDate using DateLayout.
func (r UpdateCycleRequest) ParseEndDate() (time.Time, error) {
	return time.Parse(DateLayout, r.EndDate)
}

// CycleResponse is the JSON representation of a Cycle returned to clients.
type CycleResponse struct {
	ID           string  `json:"id"`
	StartDate    string  `json:"start_date"`
	EndDate      *string `json:"end_date,omitempty"`
	CycleLength  *int    `json:"cycle_length,omitempty"`
	PeriodLength *int    `json:"period_length,omitempty"`
	IsOutlier    bool    `json:"is_outlier"`
	CreatedAt    string  `json:"created_at"`
}

// CycleWithPredictionResponse wraps a just-written cycle together with the
// freshly recalculated prediction, per BE-4.3 — every cycle write response
// includes the up-to-date prediction so the frontend can reschedule
// notifications without a second round-trip.
type CycleWithPredictionResponse struct {
	Cycle      CycleResponse      `json:"cycle"`
	Prediction PredictionResponse `json:"prediction"`
}

// ToResponse converts a domain Cycle into its API response shape.
func (c Cycle) ToResponse() CycleResponse {
	resp := CycleResponse{
		ID:           c.ID.String(),
		StartDate:    c.StartDate.Format(DateLayout),
		CycleLength:  c.CycleLength,
		PeriodLength: c.PeriodLength,
		IsOutlier:    c.IsOutlier,
		CreatedAt:    c.CreatedAt.Format(time.RFC3339),
	}
	if c.EndDate != nil {
		s := c.EndDate.Format(DateLayout)
		resp.EndDate = &s
	}
	return resp
}
