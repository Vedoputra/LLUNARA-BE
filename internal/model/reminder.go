package model

import (
	"time"

	"github.com/google/uuid"
)

// ReminderType enumerates the kinds of reminder preferences a user can set.
// The backend only stores preferences — actual scheduling/display happens
// as a local notification on-device via expo-notifications, per PRD ADR-003.
type ReminderType string

const (
	ReminderPeriodUpcoming ReminderType = "period_upcoming"
	ReminderFertileWindow  ReminderType = "fertile_window"
	ReminderMedication     ReminderType = "medication"
	ReminderCheckup        ReminderType = "checkup"
)

// Reminder is the domain representation of a user's reminder preference.
type Reminder struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	Type          ReminderType
	IsEnabled     bool
	TimeOfDay     *string // "HH:MM", nullable
	DaysBefore    *int
	CustomMessage *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// UpsertReminderRequest is the payload for PUT /api/v1/reminders.
type UpsertReminderRequest struct {
	Type          string  `json:"type" validate:"required,oneof=period_upcoming fertile_window medication checkup"`
	IsEnabled     bool    `json:"is_enabled"`
	TimeOfDay     *string `json:"time_of_day,omitempty" validate:"omitempty,datetime=15:04"`
	DaysBefore    *int    `json:"days_before,omitempty" validate:"omitempty,min=0,max=14"`
	CustomMessage *string `json:"custom_message,omitempty" validate:"omitempty,max=200"`
}

// ReminderResponse is the JSON representation of a Reminder.
type ReminderResponse struct {
	ID            string  `json:"id"`
	Type          string  `json:"type"`
	IsEnabled     bool    `json:"is_enabled"`
	TimeOfDay     *string `json:"time_of_day,omitempty"`
	DaysBefore    *int    `json:"days_before,omitempty"`
	CustomMessage *string `json:"custom_message,omitempty"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

// ToResponse converts a domain Reminder into its API response shape.
func (r Reminder) ToResponse() ReminderResponse {
	return ReminderResponse{
		ID:            r.ID.String(),
		Type:          string(r.Type),
		IsEnabled:     r.IsEnabled,
		TimeOfDay:     r.TimeOfDay,
		DaysBefore:    r.DaysBefore,
		CustomMessage: r.CustomMessage,
		CreatedAt:     r.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     r.UpdatedAt.Format(time.RFC3339),
	}
}
