package model

import (
	"time"

	"github.com/google/uuid"
)

// FlowIntensity enumerates menstrual flow levels for a logged day.
type FlowIntensity string

const (
	FlowIntensityLight  FlowIntensity = "light"
	FlowIntensityMedium FlowIntensity = "medium"
	FlowIntensityHeavy  FlowIntensity = "heavy"
)

// DailyLog is the domain representation of a single day's log.
type DailyLog struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	CycleID       *uuid.UUID
	Date          time.Time
	FlowIntensity *FlowIntensity
	Mood          *string
	Notes         *string
	SymptomIDs    []uuid.UUID
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// UpsertDailyLogRequest is the payload for POST /api/v1/daily-logs.
type UpsertDailyLogRequest struct {
	Date          string   `json:"date" validate:"required,datetime=2006-01-02"`
	FlowIntensity *string  `json:"flow_intensity,omitempty" validate:"omitempty,oneof=light medium heavy"`
	Mood          *string  `json:"mood,omitempty"`
	Notes         *string  `json:"notes,omitempty" validate:"omitempty,max=500"`
	SymptomIDs    []string `json:"symptom_ids,omitempty" validate:"omitempty,dive,uuid"`
}

// ParseDate parses Date using DateLayout.
func (r UpsertDailyLogRequest) ParseDate() (time.Time, error) {
	return time.Parse(DateLayout, r.Date)
}

// DailyLogResponse is the JSON representation of a DailyLog.
type DailyLogResponse struct {
	ID            string   `json:"id"`
	Date          string   `json:"date"`
	CycleID       *string  `json:"cycle_id,omitempty"`
	FlowIntensity *string  `json:"flow_intensity,omitempty"`
	Mood          *string  `json:"mood,omitempty"`
	Notes         *string  `json:"notes,omitempty"`
	SymptomIDs    []string `json:"symptom_ids,omitempty"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
}

// ToResponse converts a domain DailyLog into its API response shape.
func (d DailyLog) ToResponse() DailyLogResponse {
	resp := DailyLogResponse{
		ID:        d.ID.String(),
		Date:      d.Date.Format(DateLayout),
		Mood:      d.Mood,
		Notes:     d.Notes,
		CreatedAt: d.CreatedAt.Format(time.RFC3339),
		UpdatedAt: d.UpdatedAt.Format(time.RFC3339),
	}
	if d.CycleID != nil {
		s := d.CycleID.String()
		resp.CycleID = &s
	}
	if d.FlowIntensity != nil {
		s := string(*d.FlowIntensity)
		resp.FlowIntensity = &s
	}
	if len(d.SymptomIDs) > 0 {
		ids := make([]string, len(d.SymptomIDs))
		for i, id := range d.SymptomIDs {
			ids[i] = id.String()
		}
		resp.SymptomIDs = ids
	}
	return resp
}
