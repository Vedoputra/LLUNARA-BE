package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/apierror"
	"github.com/Vedoputra/LLUNARA-BE/internal/repository"
)

// maxExportRangeDays caps export requests at ~2 years, per BE-6.2 step 5.
const maxExportRangeDays = 366 * 2

// dailyLogExportRepository is the subset of *repository.ExportRepository
// that ExportService depends on.
type dailyLogExportRepository interface {
	ListDailyLogsForExport(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]repository.DailyLogExportRow, error)
}

type ExportService struct {
	exportRepo   dailyLogExportRepository
	wellnessRepo wellnessRepository
	cycleRepo    cycleRepository
}

func NewExportService(exportRepo dailyLogExportRepository, wellnessRepo wellnessRepository, cycleRepo cycleRepository) *ExportService {
	return &ExportService{exportRepo: exportRepo, wellnessRepo: wellnessRepo, cycleRepo: cycleRepo}
}

// ValidateRange enforces the shared from/to rules for every export format.
func ValidateExportRange(from, to time.Time) error {
	if to.Before(from) {
		return apierror.ValidationError("Rentang tanggal tidak valid", map[string]any{"to": "harus setelah atau sama dengan from"})
	}
	if to.Sub(from) > maxExportRangeDays*24*time.Hour {
		return apierror.ValidationError("Rentang tanggal terlalu panjang", map[string]any{"range": fmt.Sprintf("maksimal %d hari (2 tahun)", maxExportRangeDays)})
	}
	return nil
}

type exportRow struct {
	date          time.Time
	dayOfCycle    string
	phase         string
	flowIntensity string
	mood          string
	symptoms      string
	notes         string
	waterGlasses  string
	sleepHours    string
	weightKg      string
}

// buildExportRows merges daily logs and wellness logs for [from, to] into
// one row per day that has any data at all, sorted chronologically.
func (s *ExportService) buildExportRows(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]exportRow, error) {
	dailyLogs, err := s.exportRepo.ListDailyLogsForExport(ctx, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("list daily logs: %w", err)
	}
	wellnessLogs, err := s.wellnessRepo.ListByRange(ctx, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("list wellness logs: %w", err)
	}
	fallbackCycle, fallbackPeriod := averageLengthsForUser(ctx, s.cycleRepo, userID)

	wellnessByDate := make(map[string]model.WellnessLog, len(wellnessLogs))
	for _, w := range wellnessLogs {
		wellnessByDate[w.Date.Format(model.DateLayout)] = w
	}

	formatWellness := func(w model.WellnessLog) (water, sleep, weight string) {
		if w.WaterGlasses != nil {
			water = strconv.Itoa(*w.WaterGlasses)
		}
		if w.SleepHours != nil {
			sleep = strconv.FormatFloat(*w.SleepHours, 'f', 1, 64)
		}
		if w.WeightKg != nil {
			weight = strconv.FormatFloat(*w.WeightKg, 'f', 2, 64)
		}
		return water, sleep, weight
	}

	rows := make([]exportRow, 0, len(dailyLogs))
	for _, log := range dailyLogs {
		key := log.Date.Format(model.DateLayout)

		row := exportRow{date: log.Date}
		if log.Cycle != nil {
			row.dayOfCycle = strconv.Itoa(daysBetween(log.Cycle.StartDate, log.Date) + 1)
			if phase, ok := classifyOccurrencePhase(log.Date, log.Cycle, fallbackCycle, fallbackPeriod); ok {
				row.phase = string(phase)
			}
		}
		if log.FlowIntensity != nil {
			row.flowIntensity = *log.FlowIntensity
		}
		if log.Mood != nil {
			row.mood = *log.Mood
		}
		if log.Notes != nil {
			row.notes = *log.Notes
		}
		row.symptoms = strings.Join(log.SymptomNames, "; ")

		if w, ok := wellnessByDate[key]; ok {
			row.waterGlasses, row.sleepHours, row.weightKg = formatWellness(w)
			delete(wellnessByDate, key) // consumed — don't emit a duplicate wellness-only row below
		}

		rows = append(rows, row)
	}

	// Days with wellness data but no daily log still deserve a row.
	remainingKeys := make([]string, 0, len(wellnessByDate))
	for k := range wellnessByDate {
		remainingKeys = append(remainingKeys, k)
	}
	sort.Strings(remainingKeys)
	for _, k := range remainingKeys {
		w := wellnessByDate[k]
		row := exportRow{date: w.Date}
		row.waterGlasses, row.sleepHours, row.weightKg = formatWellness(w)
		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].date.Before(rows[j].date) })
	return rows, nil
}

// GenerateCSV renders the raw log data for [from, to] as CSV bytes.
func (s *ExportService) GenerateCSV(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]byte, error) {
	if err := ValidateExportRange(from, to); err != nil {
		return nil, err
	}

	rows, err := s.buildExportRows(ctx, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("build export rows: %w", err)
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	header := []string{
		"Tanggal", "Hari Siklus", "Fase", "Intensitas Flow", "Mood",
		"Gejala", "Catatan", "Air Minum (gelas)", "Tidur (jam)", "Berat (kg)",
	}
	if err := w.Write(header); err != nil {
		return nil, fmt.Errorf("write csv header: %w", err)
	}

	for _, r := range rows {
		record := []string{
			r.date.Format(model.DateLayout), r.dayOfCycle, r.phase, r.flowIntensity, r.mood,
			r.symptoms, r.notes, r.waterGlasses, r.sleepHours, r.weightKg,
		}
		if err := w.Write(record); err != nil {
			return nil, fmt.Errorf("write csv row: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("flush csv: %w", err)
	}
	return buf.Bytes(), nil
}
