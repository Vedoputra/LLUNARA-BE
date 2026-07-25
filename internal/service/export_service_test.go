package service

import (
	"context"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
	"github.com/Vedoputra/LLUNARA-BE/internal/repository"
)

type fakeExportRepo struct {
	rows []repository.DailyLogExportRow
}

func (f *fakeExportRepo) ListDailyLogsForExport(context.Context, uuid.UUID, time.Time, time.Time) ([]repository.DailyLogExportRow, error) {
	return f.rows, nil
}

type fakeSummaryProvider struct {
	summary *model.CycleSummary
}

func newFakeSummaryProvider() *fakeSummaryProvider {
	return &fakeSummaryProvider{summary: &model.CycleSummary{
		HasSufficientData:  true,
		AverageCycleLength: 28,
		TotalCycles:        3,
		Regularity:         model.RegularityRegular,
	}}
}

func (f *fakeSummaryProvider) GetCycleSummary(context.Context, uuid.UUID) (*model.CycleSummary, error) {
	return f.summary, nil
}

func parseCSV(t *testing.T, b []byte) [][]string {
	t.Helper()
	records, err := csv.NewReader(strings.NewReader(string(b))).ReadAll()
	if err != nil {
		t.Fatalf("parse generated csv: %v", err)
	}
	return records
}

func TestValidateExportRange_RejectsToBeforeFrom(t *testing.T) {
	err := ValidateExportRange(date(2026, 2, 1), date(2026, 1, 1))
	if code := apiErrCode(t, err); code != "VALIDATION_ERROR" {
		t.Errorf("code = %q, want VALIDATION_ERROR", code)
	}
}

func TestValidateExportRange_RejectsOverTwoYears(t *testing.T) {
	err := ValidateExportRange(date(2020, 1, 1), date(2026, 1, 1))
	if code := apiErrCode(t, err); code != "VALIDATION_ERROR" {
		t.Errorf("code = %q, want VALIDATION_ERROR", code)
	}
}

func TestValidateExportRange_AllowsExactlyTwoYears(t *testing.T) {
	if err := ValidateExportRange(date(2024, 1, 1), date(2026, 1, 1)); err != nil {
		t.Errorf("expected 2-year range to be allowed, got: %v", err)
	}
}

func TestGenerateCSV_HeaderAndBasicRow(t *testing.T) {
	flow := "medium"
	mood := "senang"
	notes := "catatan hari ini"
	exportRepo := &fakeExportRepo{rows: []repository.DailyLogExportRow{
		{
			ID: uuid.New(), Date: date(2026, 1, 1),
			FlowIntensity: &flow, Mood: &mood, Notes: &notes,
			SymptomNames: []string{"kram", "sakit kepala"},
		},
	}}
	svc := NewExportService(exportRepo, newFakeWellnessRepo(), newFakeCycleRepo(), newFakeSummaryProvider())

	csvBytes, err := svc.GenerateCSV(context.Background(), testUUID, date(2026, 1, 1), date(2026, 1, 31))
	if err != nil {
		t.Fatalf("GenerateCSV: %v", err)
	}
	records := parseCSV(t, csvBytes)
	if len(records) != 2 {
		t.Fatalf("expected header + 1 data row, got %d rows", len(records))
	}
	if records[0][0] != "Tanggal" {
		t.Errorf("expected header row, got %v", records[0])
	}
	dataRow := records[1]
	if dataRow[0] != "2026-01-01" {
		t.Errorf("date = %q, want 2026-01-01", dataRow[0])
	}
	if dataRow[3] != "medium" {
		t.Errorf("flow intensity = %q, want medium", dataRow[3])
	}
	if dataRow[4] != "senang" {
		t.Errorf("mood = %q, want senang", dataRow[4])
	}
	if dataRow[5] != "kram; sakit kepala" {
		t.Errorf("symptoms = %q, want %q", dataRow[5], "kram; sakit kepala")
	}
	if dataRow[6] != "catatan hari ini" {
		t.Errorf("notes = %q, want %q", dataRow[6], "catatan hari ini")
	}
}

func TestGenerateCSV_MergesLogAndWellnessOnSameDay(t *testing.T) {
	mood := "tenang"
	exportRepo := &fakeExportRepo{rows: []repository.DailyLogExportRow{
		{ID: uuid.New(), Date: date(2026, 1, 5), Mood: &mood},
	}}
	glasses := 6
	wellnessRepo := newFakeWellnessRepo()
	wellnessRepo.logs[wellnessKey(testUUID, date(2026, 1, 5))] = model.WellnessLog{
		UserID: testUUID, Date: date(2026, 1, 5), WaterGlasses: &glasses,
	}
	svc := NewExportService(exportRepo, wellnessRepo, newFakeCycleRepo(), newFakeSummaryProvider())

	csvBytes, err := svc.GenerateCSV(context.Background(), testUUID, date(2026, 1, 1), date(2026, 1, 31))
	if err != nil {
		t.Fatalf("GenerateCSV: %v", err)
	}
	records := parseCSV(t, csvBytes)
	if len(records) != 2 {
		t.Fatalf("expected exactly 1 merged data row (not 2 separate rows), got %d data rows", len(records)-1)
	}
	dataRow := records[1]
	if dataRow[4] != "tenang" {
		t.Errorf("mood = %q, want tenang", dataRow[4])
	}
	if dataRow[7] != "6" {
		t.Errorf("water_glasses = %q, want 6", dataRow[7])
	}
}

func TestGenerateCSV_WellnessOnlyDayStillGetsARow(t *testing.T) {
	exportRepo := &fakeExportRepo{} // no daily logs at all
	weight := 55.5
	wellnessRepo := newFakeWellnessRepo()
	wellnessRepo.logs[wellnessKey(testUUID, date(2026, 1, 10))] = model.WellnessLog{
		UserID: testUUID, Date: date(2026, 1, 10), WeightKg: &weight,
	}
	svc := NewExportService(exportRepo, wellnessRepo, newFakeCycleRepo(), newFakeSummaryProvider())

	csvBytes, err := svc.GenerateCSV(context.Background(), testUUID, date(2026, 1, 1), date(2026, 1, 31))
	if err != nil {
		t.Fatalf("GenerateCSV: %v", err)
	}
	records := parseCSV(t, csvBytes)
	if len(records) != 2 {
		t.Fatalf("expected 1 data row for the wellness-only day, got %d", len(records)-1)
	}
	if records[1][9] != "55.50" {
		t.Errorf("weight_kg = %q, want 55.50", records[1][9])
	}
}

func TestGenerateCSV_IncludesDayOfCycleAndPhaseWhenCycleKnown(t *testing.T) {
	cycle := &model.Cycle{StartDate: date(2026, 1, 1)}
	exportRepo := &fakeExportRepo{rows: []repository.DailyLogExportRow{
		{ID: uuid.New(), Date: date(2026, 1, 1), Cycle: cycle}, // day 1 -> menstrual
	}}
	svc := NewExportService(exportRepo, newFakeWellnessRepo(), newFakeCycleRepo(), newFakeSummaryProvider())

	csvBytes, err := svc.GenerateCSV(context.Background(), testUUID, date(2026, 1, 1), date(2026, 1, 31))
	if err != nil {
		t.Fatalf("GenerateCSV: %v", err)
	}
	records := parseCSV(t, csvBytes)
	dataRow := records[1]
	if dataRow[1] != "1" {
		t.Errorf("day_of_cycle = %q, want 1", dataRow[1])
	}
	if dataRow[2] != "menstrual" {
		t.Errorf("phase = %q, want menstrual", dataRow[2])
	}
}

func TestTopSymptoms_RanksByFrequencyAndRespectsLimit(t *testing.T) {
	rows := []exportRow{
		{symptoms: "kram; sakit kepala"},
		{symptoms: "kram"},
		{symptoms: "kram; mual"},
		{symptoms: ""},
	}

	top := topSymptoms(rows, 2)
	if len(top) != 2 {
		t.Fatalf("expected 2 results, got %d", len(top))
	}
	if top[0].name != "kram" || top[0].count != 3 {
		t.Errorf("top[0] = %+v, want kram/3", top[0])
	}
}

func TestGeneratePDF_ProducesNonEmptyPDF(t *testing.T) {
	flow := "medium"
	exportRepo := &fakeExportRepo{rows: []repository.DailyLogExportRow{
		{ID: uuid.New(), Date: date(2026, 1, 1), FlowIntensity: &flow, SymptomNames: []string{"kram"}},
	}}
	cycleRepo := newFakeCycleRepo(model.Cycle{UserID: testUUID, StartDate: date(2026, 1, 1), CycleLength: intPtr(28), PeriodLength: intPtr(5)})
	svc := NewExportService(exportRepo, newFakeWellnessRepo(), cycleRepo, newFakeSummaryProvider())

	pdfBytes, err := svc.GeneratePDF(context.Background(), testUUID, date(2026, 1, 1), date(2026, 1, 31))
	if err != nil {
		t.Fatalf("GeneratePDF: %v", err)
	}
	if len(pdfBytes) < 100 {
		t.Fatalf("expected a non-trivial PDF, got %d bytes", len(pdfBytes))
	}
	if string(pdfBytes[:4]) != "%PDF" {
		t.Errorf("expected PDF magic header, got %q", pdfBytes[:4])
	}
}

func TestGeneratePDF_RejectsInvalidRange(t *testing.T) {
	svc := NewExportService(&fakeExportRepo{}, newFakeWellnessRepo(), newFakeCycleRepo(), newFakeSummaryProvider())

	_, err := svc.GeneratePDF(context.Background(), testUUID, date(2026, 2, 1), date(2026, 1, 1))
	if code := apiErrCode(t, err); code != "VALIDATION_ERROR" {
		t.Errorf("code = %q, want VALIDATION_ERROR", code)
	}
}

func intPtr(i int) *int { return &i }
