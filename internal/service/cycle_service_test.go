package service

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/apierror"
	"github.com/Vedoputra/LLUNARA-BE/internal/repository"
)

// fakeCycleRepo is an in-memory stand-in for *repository.CycleRepository
// that mirrors its user_id-scoping and overlap semantics closely enough for
// CycleService's business logic to be tested without a real database.
type fakeCycleRepo struct {
	cycles map[uuid.UUID]model.Cycle
}

func newFakeCycleRepo(cycles ...model.Cycle) *fakeCycleRepo {
	m := make(map[uuid.UUID]model.Cycle)
	for _, c := range cycles {
		m[c.ID] = c
	}
	return &fakeCycleRepo{cycles: m}
}

func (f *fakeCycleRepo) Create(_ context.Context, cycle model.Cycle) (*model.Cycle, error) {
	cycle.ID = uuid.New()
	now := time.Now()
	cycle.CreatedAt, cycle.UpdatedAt = now, now
	f.cycles[cycle.ID] = cycle
	c := cycle
	return &c, nil
}

func (f *fakeCycleRepo) GetByID(_ context.Context, userID, cycleID uuid.UUID) (*model.Cycle, error) {
	c, ok := f.cycles[cycleID]
	if !ok || c.UserID != userID {
		return nil, repository.ErrNotFound
	}
	cc := c
	return &cc, nil
}

func (f *fakeCycleRepo) ListByUser(_ context.Context, userID uuid.UUID, limit int) ([]model.Cycle, error) {
	list := make([]model.Cycle, 0)
	for _, c := range f.cycles {
		if c.UserID == userID {
			list = append(list, c)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].StartDate.After(list[j].StartDate) })
	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}
	return list, nil
}

func (f *fakeCycleRepo) GetLatest(ctx context.Context, userID uuid.UUID) (*model.Cycle, error) {
	list, _ := f.ListByUser(ctx, userID, 1)
	if len(list) == 0 {
		return nil, repository.ErrNotFound
	}
	return &list[0], nil
}

func (f *fakeCycleRepo) FindOverlapping(_ context.Context, userID uuid.UUID, startDate time.Time) (*model.Cycle, error) {
	for _, c := range f.cycles {
		if c.UserID != userID {
			continue
		}
		if !c.StartDate.After(startDate) && (c.EndDate == nil || !c.EndDate.Before(startDate)) {
			cc := c
			return &cc, nil
		}
	}
	return nil, nil
}

func (f *fakeCycleRepo) Update(_ context.Context, cycle model.Cycle) (*model.Cycle, error) {
	existing, ok := f.cycles[cycle.ID]
	if !ok || existing.UserID != cycle.UserID {
		return nil, repository.ErrNotFound
	}
	cycle.CreatedAt = existing.CreatedAt
	cycle.UpdatedAt = time.Now()
	f.cycles[cycle.ID] = cycle
	cc := cycle
	return &cc, nil
}

func (f *fakeCycleRepo) Delete(_ context.Context, userID, cycleID uuid.UUID) error {
	c, ok := f.cycles[cycleID]
	if !ok || c.UserID != userID {
		return repository.ErrNotFound
	}
	delete(f.cycles, cycleID)
	return nil
}

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func apiErrCode(t *testing.T, err error) string {
	t.Helper()
	apiErr, ok := err.(*apierror.APIError)
	if !ok {
		t.Fatalf("expected *apierror.APIError, got %T (%v)", err, err)
	}
	return apiErr.Code
}

func TestStartCycle_FirstCycleEver(t *testing.T) {
	repo := newFakeCycleRepo()
	svc := NewCycleService(repo)
	userID := uuid.New()

	created, err := svc.StartCycle(context.Background(), userID, date(2026, 1, 1))
	if err != nil {
		t.Fatalf("StartCycle: %v", err)
	}
	if created.CycleLength != nil {
		t.Errorf("expected nil cycle_length for a brand new cycle, got %v", *created.CycleLength)
	}
	if !created.StartDate.Equal(date(2026, 1, 1)) {
		t.Errorf("start date = %v, want %v", created.StartDate, date(2026, 1, 1))
	}
}

func TestStartCycle_RejectsFutureDate(t *testing.T) {
	repo := newFakeCycleRepo()
	svc := NewCycleService(repo)

	_, err := svc.StartCycle(context.Background(), uuid.New(), time.Now().AddDate(0, 0, 5))
	if err == nil {
		t.Fatal("expected an error for a future start date")
	}
	if code := apiErrCode(t, err); code != "VALIDATION_ERROR" {
		t.Errorf("code = %q, want VALIDATION_ERROR", code)
	}
}

func TestStartCycle_RejectsDateAtOrBeforeLatest(t *testing.T) {
	userID := uuid.New()
	existing := model.Cycle{ID: uuid.New(), UserID: userID, StartDate: date(2026, 2, 1)}
	repo := newFakeCycleRepo(existing)
	svc := NewCycleService(repo)

	_, err := svc.StartCycle(context.Background(), userID, date(2026, 2, 1))
	if code := apiErrCode(t, err); code != "CYCLE_OVERLAP" {
		t.Errorf("same date as latest: code = %q, want CYCLE_OVERLAP", code)
	}

	_, err = svc.StartCycle(context.Background(), userID, date(2026, 1, 15))
	if code := apiErrCode(t, err); code != "CYCLE_OVERLAP" {
		t.Errorf("date before latest: code = %q, want CYCLE_OVERLAP", code)
	}
}

func TestStartCycle_RejectsDateWithinClosedCycle(t *testing.T) {
	userID := uuid.New()
	end := date(2026, 1, 5)
	existing := model.Cycle{ID: uuid.New(), UserID: userID, StartDate: date(2026, 1, 1), EndDate: &end}
	repo := newFakeCycleRepo(existing)
	svc := NewCycleService(repo)

	_, err := svc.StartCycle(context.Background(), userID, date(2026, 1, 3))
	if code := apiErrCode(t, err); code != "CYCLE_OVERLAP" {
		t.Errorf("code = %q, want CYCLE_OVERLAP", code)
	}
}

func TestStartCycle_ClosesPreviousOpenCycle_NormalLength(t *testing.T) {
	userID := uuid.New()
	previous := model.Cycle{ID: uuid.New(), UserID: userID, StartDate: date(2026, 1, 1)}
	repo := newFakeCycleRepo(previous)
	svc := NewCycleService(repo)

	_, err := svc.StartCycle(context.Background(), userID, date(2026, 1, 29)) // 28 days later
	if err != nil {
		t.Fatalf("StartCycle: %v", err)
	}

	closed := repo.cycles[previous.ID]
	if closed.CycleLength == nil || *closed.CycleLength != 28 {
		t.Fatalf("previous cycle_length = %v, want 28", closed.CycleLength)
	}
	if closed.IsOutlier {
		t.Error("28-day cycle should not be flagged as an outlier")
	}
}

func TestStartCycle_ClosesPreviousOpenCycle_FlagsOutliers(t *testing.T) {
	tests := []struct {
		name          string
		gapDays       int
		wantIsOutlier bool
	}{
		{"too short (10 days)", 10, true},
		{"boundary short (21 days)", 21, false},
		{"boundary long (45 days)", 45, false},
		{"too long (60 days)", 60, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := uuid.New()
			previous := model.Cycle{ID: uuid.New(), UserID: userID, StartDate: date(2026, 1, 1)}
			repo := newFakeCycleRepo(previous)
			svc := NewCycleService(repo)

			newStart := date(2026, 1, 1).AddDate(0, 0, tt.gapDays)
			if _, err := svc.StartCycle(context.Background(), userID, newStart); err != nil {
				t.Fatalf("StartCycle: %v", err)
			}

			closed := repo.cycles[previous.ID]
			if closed.IsOutlier != tt.wantIsOutlier {
				t.Errorf("is_outlier = %v, want %v (gap=%d days)", closed.IsOutlier, tt.wantIsOutlier, tt.gapDays)
			}
		})
	}
}

func TestStartCycle_ScopedToUser(t *testing.T) {
	userA := uuid.New()
	userB := uuid.New()
	// userA has a cycle; userB starting a cycle on the same date must not
	// be blocked by userA's data.
	repo := newFakeCycleRepo(model.Cycle{ID: uuid.New(), UserID: userA, StartDate: date(2026, 1, 1)})
	svc := NewCycleService(repo)

	_, err := svc.StartCycle(context.Background(), userB, date(2026, 1, 1))
	if err != nil {
		t.Fatalf("expected user B's cycle to succeed independently of user A's data, got: %v", err)
	}
}

func TestEndCycle_SetsPeriodLength(t *testing.T) {
	userID := uuid.New()
	cycle := model.Cycle{ID: uuid.New(), UserID: userID, StartDate: date(2026, 1, 1)}
	repo := newFakeCycleRepo(cycle)
	svc := NewCycleService(repo)

	updated, err := svc.EndCycle(context.Background(), userID, cycle.ID, date(2026, 1, 5))
	if err != nil {
		t.Fatalf("EndCycle: %v", err)
	}
	if updated.PeriodLength == nil || *updated.PeriodLength != 5 {
		t.Errorf("period_length = %v, want 5", updated.PeriodLength)
	}
	if updated.EndDate == nil || !updated.EndDate.Equal(date(2026, 1, 5)) {
		t.Errorf("end_date = %v, want %v", updated.EndDate, date(2026, 1, 5))
	}
}

func TestEndCycle_RejectsEndBeforeStart(t *testing.T) {
	userID := uuid.New()
	cycle := model.Cycle{ID: uuid.New(), UserID: userID, StartDate: date(2026, 1, 10)}
	repo := newFakeCycleRepo(cycle)
	svc := NewCycleService(repo)

	_, err := svc.EndCycle(context.Background(), userID, cycle.ID, date(2026, 1, 5))
	if code := apiErrCode(t, err); code != "VALIDATION_ERROR" {
		t.Errorf("code = %q, want VALIDATION_ERROR", code)
	}
}

func TestEndCycle_AllowsOutOfRangeDurationWithoutError(t *testing.T) {
	userID := uuid.New()
	cycle := model.Cycle{ID: uuid.New(), UserID: userID, StartDate: date(2026, 1, 1)}
	repo := newFakeCycleRepo(cycle)
	svc := NewCycleService(repo)

	// 20 days is well outside the normal 1-14 day range but must still be
	// allowed — it's tracked, not gated.
	updated, err := svc.EndCycle(context.Background(), userID, cycle.ID, date(2026, 1, 20))
	if err != nil {
		t.Fatalf("expected out-of-range period duration to still be allowed, got error: %v", err)
	}
	if updated.PeriodLength == nil || *updated.PeriodLength != 20 {
		t.Errorf("period_length = %v, want 20", updated.PeriodLength)
	}
}

func TestEndCycle_WrongUserIsNotFound(t *testing.T) {
	cycle := model.Cycle{ID: uuid.New(), UserID: uuid.New(), StartDate: date(2026, 1, 1)}
	repo := newFakeCycleRepo(cycle)
	svc := NewCycleService(repo)

	_, err := svc.EndCycle(context.Background(), uuid.New(), cycle.ID, date(2026, 1, 5))
	if code := apiErrCode(t, err); code != "NOT_FOUND" {
		t.Errorf("code = %q, want NOT_FOUND", code)
	}
}

func TestDeleteCycle_MiddleOfHistory_RecalculatesOlderNeighbor(t *testing.T) {
	userID := uuid.New()
	c1 := model.Cycle{ID: uuid.New(), UserID: userID, StartDate: date(2026, 1, 1)}
	length1 := 29
	c1.CycleLength = &length1
	c2 := model.Cycle{ID: uuid.New(), UserID: userID, StartDate: date(2026, 1, 30)}
	length2 := 28
	c2.CycleLength = &length2
	c3 := model.Cycle{ID: uuid.New(), UserID: userID, StartDate: date(2026, 2, 27)} // latest, still open

	repo := newFakeCycleRepo(c1, c2, c3)
	svc := NewCycleService(repo)

	if err := svc.DeleteCycle(context.Background(), userID, c2.ID); err != nil {
		t.Fatalf("DeleteCycle: %v", err)
	}

	if _, ok := repo.cycles[c2.ID]; ok {
		t.Fatal("expected the deleted cycle to be gone")
	}

	updatedC1 := repo.cycles[c1.ID]
	wantLength := daysBetween(c1.StartDate, c3.StartDate) // 1 Jan -> 27 Feb
	if updatedC1.CycleLength == nil || *updatedC1.CycleLength != wantLength {
		t.Errorf("c1 cycle_length = %v, want %d (recalculated to bridge to c3)", updatedC1.CycleLength, wantLength)
	}
}

func TestDeleteCycle_DeletingLatest_ReopensOlderNeighbor(t *testing.T) {
	userID := uuid.New()
	c1 := model.Cycle{ID: uuid.New(), UserID: userID, StartDate: date(2026, 1, 1)}
	length1 := 29
	c1.CycleLength = &length1
	c2 := model.Cycle{ID: uuid.New(), UserID: userID, StartDate: date(2026, 1, 30)} // latest, open

	repo := newFakeCycleRepo(c1, c2)
	svc := NewCycleService(repo)

	if err := svc.DeleteCycle(context.Background(), userID, c2.ID); err != nil {
		t.Fatalf("DeleteCycle: %v", err)
	}

	updatedC1 := repo.cycles[c1.ID]
	if updatedC1.CycleLength != nil {
		t.Errorf("expected c1 to become reopened (cycle_length nil) after deleting the newer cycle, got %v", *updatedC1.CycleLength)
	}
	if updatedC1.IsOutlier {
		t.Error("reopened cycle should not remain flagged as an outlier")
	}
}

func TestDeleteCycle_OnlyCycle_NoNeighborToUpdate(t *testing.T) {
	userID := uuid.New()
	c1 := model.Cycle{ID: uuid.New(), UserID: userID, StartDate: date(2026, 1, 1)}
	repo := newFakeCycleRepo(c1)
	svc := NewCycleService(repo)

	if err := svc.DeleteCycle(context.Background(), userID, c1.ID); err != nil {
		t.Fatalf("DeleteCycle: %v", err)
	}
	if len(repo.cycles) != 0 {
		t.Errorf("expected no cycles left, got %d", len(repo.cycles))
	}
}

func TestDeleteCycle_NotFound(t *testing.T) {
	repo := newFakeCycleRepo()
	svc := NewCycleService(repo)

	err := svc.DeleteCycle(context.Background(), uuid.New(), uuid.New())
	if code := apiErrCode(t, err); code != "NOT_FOUND" {
		t.Errorf("code = %q, want NOT_FOUND", code)
	}
}

func TestDeleteCycle_WrongUserIsNotFound(t *testing.T) {
	cycle := model.Cycle{ID: uuid.New(), UserID: uuid.New(), StartDate: date(2026, 1, 1)}
	repo := newFakeCycleRepo(cycle)
	svc := NewCycleService(repo)

	err := svc.DeleteCycle(context.Background(), uuid.New(), cycle.ID)
	if code := apiErrCode(t, err); code != "NOT_FOUND" {
		t.Errorf("code = %q, want NOT_FOUND", code)
	}
	if _, err := repo.GetByID(context.Background(), cycle.UserID, cycle.ID); err != nil {
		t.Error("cycle should NOT have been deleted when the wrong user attempted it")
	}
}
