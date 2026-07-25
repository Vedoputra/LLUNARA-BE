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
	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/line"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/consts/pagesize"
	"github.com/johnfercher/maroto/v2/pkg/props"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/apierror"
	"github.com/Vedoputra/LLUNARA-BE/internal/repository"
)

// maxExportRangeDays caps export requests at ~2 years, per BE-6.2 step 5.
const maxExportRangeDays = 366 * 2

// exportDisclaimer is the exact medical-disclaimer text required on every
// PDF export footer, per PRD.md section 14 — it must not be paraphrased.
const exportDisclaimer = "LLunara adalah alat bantu pencatatan pribadi. Prediksi yang ditampilkan merupakan estimasi berdasarkan data yang kamu masukkan, dan bukan metode kontrasepsi, bukan diagnosis, serta bukan pengganti konsultasi dengan tenaga medis. Untuk kekhawatiran terkait kesehatan, silakan berkonsultasi dengan dokter atau bidan."

// dailyLogExportRepository is the subset of *repository.ExportRepository
// that ExportService depends on.
type dailyLogExportRepository interface {
	ListDailyLogsForExport(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]repository.DailyLogExportRow, error)
}

// cycleSummaryProvider is the subset of *InsightService that ExportService
// depends on, so the PDF's summary section reuses the exact same stats
// (and regularity thresholds) as GET /api/v1/insights/summary instead of
// recomputing them.
type cycleSummaryProvider interface {
	GetCycleSummary(ctx context.Context, userID uuid.UUID) (*model.CycleSummary, error)
}

type ExportService struct {
	exportRepo     dailyLogExportRepository
	wellnessRepo   wellnessRepository
	cycleRepo      cycleRepository
	insightService cycleSummaryProvider
}

func NewExportService(exportRepo dailyLogExportRepository, wellnessRepo wellnessRepository, cycleRepo cycleRepository, insightService cycleSummaryProvider) *ExportService {
	return &ExportService{exportRepo: exportRepo, wellnessRepo: wellnessRepo, cycleRepo: cycleRepo, insightService: insightService}
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

// symptomCount is one entry in the PDF's "most frequent symptoms" section.
type symptomCount struct {
	name  string
	count int
}

// topSymptoms tallies symptom occurrences directly from already-merged
// export rows (row.symptoms is a "; "-joined name list), avoiding a second
// query, and returns at most limit entries ranked by frequency.
func topSymptoms(rows []exportRow, limit int) []symptomCount {
	counts := make(map[string]int)
	order := make([]string, 0)
	for _, r := range rows {
		if r.symptoms == "" {
			continue
		}
		for _, name := range strings.Split(r.symptoms, "; ") {
			if _, seen := counts[name]; !seen {
				order = append(order, name)
			}
			counts[name]++
		}
	}

	result := make([]symptomCount, len(order))
	for i, name := range order {
		result[i] = symptomCount{name: name, count: counts[name]}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].count > result[j].count })
	if len(result) > limit {
		result = result[:limit]
	}
	return result
}

func regularityLabel(r model.Regularity) string {
	switch r {
	case model.RegularityRegular:
		return "Teratur"
	case model.RegularityModerate:
		return "Cukup teratur"
	case model.RegularityIrregular:
		return "Tidak teratur"
	default:
		return "-"
	}
}

// GeneratePDF renders a summary report for [from, to]: a header with the
// report period, a stats summary, a cycle history table, the most frequent
// symptoms, and a footer with the mandatory medical disclaimer.
func (s *ExportService) GeneratePDF(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]byte, error) {
	if err := ValidateExportRange(from, to); err != nil {
		return nil, err
	}

	rows, err := s.buildExportRows(ctx, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("build export rows: %w", err)
	}

	summary, err := s.insightService.GetCycleSummary(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get cycle summary: %w", err)
	}

	cycles, err := s.cycleRepo.ListByUser(ctx, userID, 100)
	if err != nil {
		return nil, fmt.Errorf("list cycles: %w", err)
	}
	rangeCycles := make([]model.Cycle, 0, len(cycles))
	for _, c := range cycles {
		if !c.StartDate.Before(from) && !c.StartDate.After(to) {
			rangeCycles = append(rangeCycles, c)
		}
	}
	sort.Slice(rangeCycles, func(i, j int) bool { return rangeCycles[i].StartDate.Before(rangeCycles[j].StartDate) })

	gray := &props.Color{Red: 110, Green: 110, Blue: 110}

	cfg := config.NewBuilder().
		WithPageSize(pagesize.A4).
		WithLeftMargin(12).
		WithTopMargin(10).
		WithRightMargin(12).
		WithBottomMargin(15).
		Build()
	m := maroto.New(cfg)

	if err := m.RegisterHeader(
		text.NewRow(9, "LLunara - Laporan Siklus", props.Text{Size: 14, Style: fontstyle.Bold, Align: align.Center}),
		text.NewRow(6, fmt.Sprintf("Periode: %s s/d %s  |  Dibuat: %s",
			from.Format(model.DateLayout), to.Format(model.DateLayout), time.Now().UTC().Format(model.DateLayout)),
			props.Text{Size: 9, Align: align.Center, Color: gray}),
	); err != nil {
		return nil, fmt.Errorf("register pdf header: %w", err)
	}

	if err := m.RegisterFooter(
		line.NewRow(3, props.Line{Color: gray}),
		text.NewAutoRow(exportDisclaimer, props.Text{Size: 7, Align: align.Center, Color: gray}),
	); err != nil {
		return nil, fmt.Errorf("register pdf footer: %w", err)
	}

	m.AddRow(8, text.NewCol(12, "Ringkasan", props.Text{Size: 12, Style: fontstyle.Bold}))
	if summary.HasSufficientData {
		m.AddRow(6,
			text.NewCol(6, fmt.Sprintf("Rata-rata panjang siklus: %.1f hari", summary.AverageCycleLength), props.Text{Size: 10}),
			text.NewCol(6, fmt.Sprintf("Keteraturan: %s", regularityLabel(summary.Regularity)), props.Text{Size: 10}),
		)
		m.AddRow(6,
			text.NewCol(6, fmt.Sprintf("Rata-rata durasi menstruasi: %.1f hari", summary.AveragePeriodLength), props.Text{Size: 10}),
			text.NewCol(6, fmt.Sprintf("Total siklus tercatat: %d", summary.TotalCycles), props.Text{Size: 10}),
		)
	} else {
		m.AddRow(6, text.NewCol(12, summary.Message, props.Text{Size: 10}))
	}

	m.AddRow(10, text.NewCol(12, "", props.Text{Size: 4}))

	m.AddRow(8, text.NewCol(12, "Riwayat Siklus", props.Text{Size: 12, Style: fontstyle.Bold}))
	if len(rangeCycles) == 0 {
		m.AddRow(6, text.NewCol(12, "Tidak ada siklus tercatat pada periode ini.", props.Text{Size: 10}))
	} else {
		m.AddRow(6,
			text.NewCol(4, "Tanggal Mulai", props.Text{Size: 9, Style: fontstyle.Bold}),
			text.NewCol(4, "Panjang Siklus", props.Text{Size: 9, Style: fontstyle.Bold}),
			text.NewCol(4, "Durasi Menstruasi", props.Text{Size: 9, Style: fontstyle.Bold}),
		)
		for _, c := range rangeCycles {
			cycleLen, periodLen := "-", "-"
			if c.CycleLength != nil {
				cycleLen = fmt.Sprintf("%d hari", *c.CycleLength)
			}
			if c.PeriodLength != nil {
				periodLen = fmt.Sprintf("%d hari", *c.PeriodLength)
			}
			m.AddRow(6,
				text.NewCol(4, c.StartDate.Format(model.DateLayout), props.Text{Size: 9}),
				text.NewCol(4, cycleLen, props.Text{Size: 9}),
				text.NewCol(4, periodLen, props.Text{Size: 9}),
			)
		}
	}

	m.AddRow(10, text.NewCol(12, "", props.Text{Size: 4}))

	m.AddRow(8, text.NewCol(12, "Gejala Paling Sering", props.Text{Size: 12, Style: fontstyle.Bold}))
	top := topSymptoms(rows, 5)
	if len(top) == 0 {
		m.AddRow(6, text.NewCol(12, "Tidak ada gejala tercatat pada periode ini.", props.Text{Size: 10}))
	} else {
		for _, sc := range top {
			m.AddRow(6,
				text.NewCol(8, sc.name, props.Text{Size: 10}),
				text.NewCol(4, fmt.Sprintf("%d kali", sc.count), props.Text{Size: 10}),
			)
		}
	}

	doc, err := m.Generate()
	if err != nil {
		return nil, fmt.Errorf("generate pdf: %w", err)
	}
	return doc.GetBytes(), nil
}
