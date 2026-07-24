package model

import (
	"time"

	"github.com/google/uuid"
)

// Phase represents which phase of the cycle a given day falls into.
type Phase string

const (
	PhaseMenstrual  Phase = "menstrual"
	PhaseFollicular Phase = "follicular"
	PhaseOvulation  Phase = "ovulation"
	PhaseLuteal     Phase = "luteal"
)

// Regularity describes how consistent a user's cycle lengths are.
type Regularity string

const (
	RegularityRegular   Regularity = "regular"
	RegularityModerate  Regularity = "moderate"
	RegularityIrregular Regularity = "irregular"
)

// CycleLengthPoint is one data point in a cycle-length-over-time trend.
type CycleLengthPoint struct {
	StartDate   time.Time
	CycleLength int
}

// CycleSummary is the aggregated result of GET /api/v1/insights/summary.
type CycleSummary struct {
	HasSufficientData   bool
	AverageCycleLength  float64
	ShortestCycle       int
	LongestCycle        int
	AveragePeriodLength float64
	TotalCycles         int
	Regularity          Regularity
	CycleLengthTrend    []CycleLengthPoint
}

type CycleLengthPointResponse struct {
	StartDate   string `json:"start_date"`
	CycleLength int    `json:"cycle_length"`
}

type CycleSummaryResponse struct {
	HasSufficientData   bool                       `json:"has_sufficient_data"`
	Message             string                     `json:"message,omitempty"`
	AverageCycleLength  *float64                   `json:"average_cycle_length,omitempty"`
	ShortestCycle       *int                       `json:"shortest_cycle,omitempty"`
	LongestCycle        *int                       `json:"longest_cycle,omitempty"`
	AveragePeriodLength *float64                   `json:"average_period_length,omitempty"`
	TotalCycles         int                        `json:"total_cycles"`
	Regularity          *string                    `json:"regularity,omitempty"`
	CycleLengthTrend    []CycleLengthPointResponse `json:"cycle_length_trend,omitempty"`
}

// ToResponse converts a domain CycleSummary into its API response shape.
func (s CycleSummary) ToResponse() CycleSummaryResponse {
	resp := CycleSummaryResponse{
		HasSufficientData: s.HasSufficientData,
		TotalCycles:       s.TotalCycles,
	}
	if !s.HasSufficientData {
		return resp
	}

	avgCycle := s.AverageCycleLength
	shortest := s.ShortestCycle
	longest := s.LongestCycle
	avgPeriod := s.AveragePeriodLength
	regularity := string(s.Regularity)

	resp.AverageCycleLength = &avgCycle
	resp.ShortestCycle = &shortest
	resp.LongestCycle = &longest
	resp.AveragePeriodLength = &avgPeriod
	resp.Regularity = &regularity

	if len(s.CycleLengthTrend) > 0 {
		trend := make([]CycleLengthPointResponse, len(s.CycleLengthTrend))
		for i, p := range s.CycleLengthTrend {
			trend[i] = CycleLengthPointResponse{
				StartDate:   p.StartDate.Format(DateLayout),
				CycleLength: p.CycleLength,
			}
		}
		resp.CycleLengthTrend = trend
	}
	return resp
}

// SymptomFrequency is one entry in GET /api/v1/insights/symptoms — how often
// a symptom was logged, ranked from most to least frequent.
type SymptomFrequency struct {
	SymptomID  uuid.UUID
	Name       string
	Count      int
	SampleSize int
}

type SymptomFrequencyResponse struct {
	SymptomID  string `json:"symptom_id"`
	Name       string `json:"name"`
	Count      int    `json:"count"`
	SampleSize int    `json:"sample_size"`
}

func (f SymptomFrequency) ToResponse() SymptomFrequencyResponse {
	return SymptomFrequencyResponse{
		SymptomID:  f.SymptomID.String(),
		Name:       f.Name,
		Count:      f.Count,
		SampleSize: f.SampleSize,
	}
}

// SymptomPhaseDistribution shows how often a symptom occurs in each cycle
// phase.
type SymptomPhaseDistribution struct {
	SymptomID  uuid.UUID
	Name       string
	ByPhase    map[Phase]int
	SampleSize int
}

type SymptomPhaseDistributionResponse struct {
	SymptomID  string         `json:"symptom_id"`
	Name       string         `json:"name"`
	ByPhase    map[string]int `json:"by_phase"`
	SampleSize int            `json:"sample_size"`
}

func (d SymptomPhaseDistribution) ToResponse() SymptomPhaseDistributionResponse {
	byPhase := make(map[string]int, len(d.ByPhase))
	for phase, count := range d.ByPhase {
		byPhase[string(phase)] = count
	}
	return SymptomPhaseDistributionResponse{
		SymptomID:  d.SymptomID.String(),
		Name:       d.Name,
		ByPhase:    byPhase,
		SampleSize: d.SampleSize,
	}
}

// MoodPhaseDistribution is the result of GET /api/v1/insights/mood for a
// single cycle phase — how often each mood was logged during it.
type MoodPhaseDistribution struct {
	Phase        Phase
	MoodCounts   map[string]int
	DominantMood string
	SampleSize   int
}

type MoodPhaseDistributionResponse struct {
	Phase        string         `json:"phase"`
	MoodCounts   map[string]int `json:"mood_counts"`
	DominantMood string         `json:"dominant_mood,omitempty"`
	SampleSize   int            `json:"sample_size"`
}

func (m MoodPhaseDistribution) ToResponse() MoodPhaseDistributionResponse {
	return MoodPhaseDistributionResponse{
		Phase:        string(m.Phase),
		MoodCounts:   m.MoodCounts,
		DominantMood: m.DominantMood,
		SampleSize:   m.SampleSize,
	}
}
