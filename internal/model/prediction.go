package model

import "time"

// Confidence describes how much history backs a prediction.
type Confidence string

const (
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

// Prediction is the domain result of running the prediction algorithm
// against a user's cycle history. The pointer fields are nil when there's
// no recorded cycle to predict from at all — not an error, just nothing to
// show yet.
type Prediction struct {
	NextPeriodStart    *time.Time
	NextPeriodEnd      *time.Time
	EstimatedOvulation *time.Time
	FertileWindowStart *time.Time
	FertileWindowEnd   *time.Time
	CurrentPhase       *Phase
	DayOfCycle         *int
	Confidence         Confidence
	BasedOnCycles      int
	AverageCycleLength int
}

type FertileWindowResponse struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// PredictionResponse is the JSON shape for GET /api/v1/cycles/prediction,
// per PRD Section 7.2.
type PredictionResponse struct {
	NextPeriodStart    *string                `json:"next_period_start"`
	NextPeriodEnd      *string                `json:"next_period_end"`
	EstimatedOvulation *string                `json:"estimated_ovulation"`
	FertileWindow      *FertileWindowResponse `json:"fertile_window"`
	CurrentPhase       *string                `json:"current_phase"`
	DayOfCycle         *int                   `json:"day_of_cycle"`
	Confidence         string                 `json:"confidence"`
	BasedOnCycles      int                    `json:"based_on_cycles"`
	AverageCycleLength int                    `json:"average_cycle_length"`
}

// ToResponse converts a domain Prediction into its API response shape.
func (p Prediction) ToResponse() PredictionResponse {
	resp := PredictionResponse{
		Confidence:         string(p.Confidence),
		BasedOnCycles:      p.BasedOnCycles,
		AverageCycleLength: p.AverageCycleLength,
		DayOfCycle:         p.DayOfCycle,
	}
	if p.NextPeriodStart != nil {
		s := p.NextPeriodStart.Format(DateLayout)
		resp.NextPeriodStart = &s
	}
	if p.NextPeriodEnd != nil {
		s := p.NextPeriodEnd.Format(DateLayout)
		resp.NextPeriodEnd = &s
	}
	if p.EstimatedOvulation != nil {
		s := p.EstimatedOvulation.Format(DateLayout)
		resp.EstimatedOvulation = &s
	}
	if p.FertileWindowStart != nil && p.FertileWindowEnd != nil {
		resp.FertileWindow = &FertileWindowResponse{
			Start: p.FertileWindowStart.Format(DateLayout),
			End:   p.FertileWindowEnd.Format(DateLayout),
		}
	}
	if p.CurrentPhase != nil {
		s := string(*p.CurrentPhase)
		resp.CurrentPhase = &s
	}
	return resp
}
