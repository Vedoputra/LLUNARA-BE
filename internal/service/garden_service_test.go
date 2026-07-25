package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeGardenRepo struct {
	total      int
	moods      []string
	rangeCalls []struct{ from, to time.Time }
}

func (f *fakeGardenRepo) CountDistinctLoggedDays(_ context.Context, _ uuid.UUID) (int, error) {
	return f.total, nil
}

func (f *fakeGardenRepo) CountDistinctLoggedDaysInRange(_ context.Context, _ uuid.UUID, from, to time.Time) (int, error) {
	f.rangeCalls = append(f.rangeCalls, struct{ from, to time.Time }{from, to})
	return 0, nil
}

func (f *fakeGardenRepo) DistinctMoods(_ context.Context, _ uuid.UUID) ([]string, error) {
	return f.moods, nil
}

func TestGetGarden_NewUserHasEmptyGardenNotError(t *testing.T) {
	repo := &fakeGardenRepo{}
	svc := NewGardenService(repo)

	garden, err := svc.GetGarden(context.Background(), testUUID)
	if err != nil {
		t.Fatalf("GetGarden: %v", err)
	}
	if garden.TotalLoggedDays != 0 || garden.LoggedDaysThisMonth != 0 || garden.NewThisWeek != 0 {
		t.Errorf("expected all-zero counts for a new user, got %+v", garden)
	}
	if len(garden.CollectedMoods) != 0 {
		t.Errorf("expected no collected moods, got %v", garden.CollectedMoods)
	}
	if len(garden.UncollectedMoods) != 7 {
		t.Errorf("expected all 7 preset moods uncollected, got %d", len(garden.UncollectedMoods))
	}
	if garden.Message == "" {
		t.Error("expected a non-empty message")
	}
}

func TestGetGarden_ReturnsTotalFromRepo(t *testing.T) {
	repo := &fakeGardenRepo{total: 34}
	svc := NewGardenService(repo)

	garden, err := svc.GetGarden(context.Background(), testUUID)
	if err != nil {
		t.Fatalf("GetGarden: %v", err)
	}
	if garden.TotalLoggedDays != 34 {
		t.Errorf("TotalLoggedDays = %d, want 34", garden.TotalLoggedDays)
	}
}

func TestGetGarden_QueriesMonthAndWeekRangesEndingToday(t *testing.T) {
	repo := &fakeGardenRepo{}
	svc := NewGardenService(repo)

	if _, err := svc.GetGarden(context.Background(), testUUID); err != nil {
		t.Fatalf("GetGarden: %v", err)
	}
	if len(repo.rangeCalls) != 2 {
		t.Fatalf("expected 2 ranged queries (month + week), got %d", len(repo.rangeCalls))
	}

	today := truncateToDate(time.Now().UTC())
	monthCall, weekCall := repo.rangeCalls[0], repo.rangeCalls[1]

	if !monthCall.to.Equal(today) || !weekCall.to.Equal(today) {
		t.Errorf("expected both ranges to end today (%v), got month.to=%v week.to=%v", today, monthCall.to, weekCall.to)
	}
	if monthCall.from.Day() != 1 {
		t.Errorf("expected month range to start on the 1st, got %v", monthCall.from)
	}
	if got := int(today.Sub(weekCall.from).Hours() / 24); got != 6 {
		t.Errorf("expected week range to span 6 days back from today, got %d days", got)
	}
}

func TestGetGarden_SplitsCollectedAndUncollectedMoods(t *testing.T) {
	repo := &fakeGardenRepo{moods: []string{"senang", "tenang"}}
	svc := NewGardenService(repo)

	garden, err := svc.GetGarden(context.Background(), testUUID)
	if err != nil {
		t.Fatalf("GetGarden: %v", err)
	}
	if len(garden.CollectedMoods) != 2 {
		t.Errorf("CollectedMoods = %v, want 2 entries", garden.CollectedMoods)
	}
	if len(garden.UncollectedMoods) != 5 {
		t.Errorf("UncollectedMoods = %v, want 5 entries", garden.UncollectedMoods)
	}
	for _, m := range garden.UncollectedMoods {
		if m == "senang" || m == "tenang" {
			t.Errorf("uncollected list should not contain an already-collected mood, got %v", garden.UncollectedMoods)
		}
	}
}

func TestGetGarden_NeverReturnsAbsenceMeasuringMessage(t *testing.T) {
	// Positive-only rule from PRD 4.5 / DESIGN.md: the message must never
	// mention missed days or a streak.
	repo := &fakeGardenRepo{}
	svc := NewGardenService(repo)

	garden, err := svc.GetGarden(context.Background(), testUUID)
	if err != nil {
		t.Fatalf("GetGarden: %v", err)
	}
	banned := []string{"streak", "bolong", "gagal", "putus"}
	lowerMsg := strings.ToLower(garden.Message)
	for _, word := range banned {
		if strings.Contains(lowerMsg, word) {
			t.Errorf("message %q must not contain negative-framing word %q", garden.Message, word)
		}
	}
}
