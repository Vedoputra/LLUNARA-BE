package model

import "github.com/google/uuid"

// Symptom is the domain representation of a symptom tag — either a system
// preset (UserID nil) or a user's custom tag.
type Symptom struct {
	ID       uuid.UUID
	UserID   *uuid.UUID
	Name     string
	Category string
	IsCustom bool
}

// CreateSymptomRequest is the payload for POST /api/v1/symptoms.
type CreateSymptomRequest struct {
	Name     string `json:"name" validate:"required,max=50"`
	Category string `json:"category" validate:"required,oneof=physical emotional other"`
}

// SymptomResponse is the JSON representation of a Symptom.
type SymptomResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	IsCustom bool   `json:"is_custom"`
}

// ToResponse converts a domain Symptom into its API response shape.
func (s Symptom) ToResponse() SymptomResponse {
	return SymptomResponse{
		ID:       s.ID.String(),
		Name:     s.Name,
		Category: s.Category,
		IsCustom: s.IsCustom,
	}
}
