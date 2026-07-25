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
	Message             string
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
		Message:           s.Message,
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

// SymptomInsight aggregates one symptom's occurrence pattern over the
// requested window: how often it was logged, which cycle phases it
// appeared in, and which day of the cycle it most commonly showed up on.
type SymptomInsight struct {
	SymptomID          uuid.UUID
	Name               string
	Count              int
	PhaseDistribution  map[Phase]int
	MostCommonCycleDay *int
	SampleSize         int
}

type SymptomInsightResponse struct {
	SymptomID          string         `json:"symptom_id"`
	Name               string         `json:"name"`
	Count              int            `json:"count"`
	PhaseDistribution  map[string]int `json:"phase_distribution"`
	MostCommonCycleDay *int           `json:"most_common_cycle_day,omitempty"`
	SampleSize         int            `json:"sample_size"`
}

func (si SymptomInsight) ToResponse() SymptomInsightResponse {
	byPhase := make(map[string]int, len(si.PhaseDistribution))
	for phase, count := range si.PhaseDistribution {
		byPhase[string(phase)] = count
	}
	return SymptomInsightResponse{
		SymptomID:          si.SymptomID.String(),
		Name:               si.Name,
		Count:              si.Count,
		PhaseDistribution:  byPhase,
		MostCommonCycleDay: si.MostCommonCycleDay,
		SampleSize:         si.SampleSize,
	}
}

// SymptomInsights is the full result of GET /api/v1/insights/symptoms,
// symptoms already ranked most to least frequent.
type SymptomInsights struct {
	Symptoms   []SymptomInsight
	Months     int
	SampleSize int
}

type SymptomInsightsResponse struct {
	Symptoms   []SymptomInsightResponse `json:"symptoms"`
	Months     int                      `json:"months"`
	SampleSize int                      `json:"sample_size"`
}

func (si SymptomInsights) ToResponse() SymptomInsightsResponse {
	symptoms := make([]SymptomInsightResponse, len(si.Symptoms))
	for i, s := range si.Symptoms {
		symptoms[i] = s.ToResponse()
	}
	return SymptomInsightsResponse{
		Symptoms:   symptoms,
		Months:     si.Months,
		SampleSize: si.SampleSize,
	}
}

// MoodPhaseDistribution is the result of GET /api/v1/insights/mood for a
// single cycle phase — how often (and what percentage of the time) each
// mood was logged during it.
type MoodPhaseDistribution struct {
	Phase          Phase
	MoodCounts     map[string]int
	MoodPercentage map[string]float64
	DominantMood   string
	SampleSize     int
}

type MoodPhaseDistributionResponse struct {
	Phase          string             `json:"phase"`
	MoodCounts     map[string]int     `json:"mood_counts"`
	MoodPercentage map[string]float64 `json:"mood_percentage"`
	DominantMood   string             `json:"dominant_mood,omitempty"`
	SampleSize     int                `json:"sample_size"`
}

func (m MoodPhaseDistribution) ToResponse() MoodPhaseDistributionResponse {
	return MoodPhaseDistributionResponse{
		Phase:          string(m.Phase),
		MoodCounts:     m.MoodCounts,
		MoodPercentage: m.MoodPercentage,
		DominantMood:   m.DominantMood,
		SampleSize:     m.SampleSize,
	}
}

// MoodInsights is the full result of GET /api/v1/insights/mood, one entry
// per cycle phase.
type MoodInsights struct {
	ByPhase []MoodPhaseDistribution
	Months  int
}

type MoodInsightsResponse struct {
	ByPhase []MoodPhaseDistributionResponse `json:"by_phase"`
	Months  int                             `json:"months"`
}

func (mi MoodInsights) ToResponse() MoodInsightsResponse {
	byPhase := make([]MoodPhaseDistributionResponse, len(mi.ByPhase))
	for i, m := range mi.ByPhase {
		byPhase[i] = m.ToResponse()
	}
	return MoodInsightsResponse{ByPhase: byPhase, Months: mi.Months}
}
