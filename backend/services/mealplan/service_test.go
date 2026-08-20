package mealplan

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/mealplan"
)

// fakeRepo is an in-memory MealPlanEntryRepository for the service unit tests.
// It records the arguments the service passes so the tests can assert on the
// exact entries produced (positions, trimmed dishes, normalized notes) without
// a database.
type fakeRepo struct {
	rangeResult []*mealplan.MealPlanEntry
	rangeErr    error
	gotStart    timezone.Date
	gotEnd      timezone.Date

	replaceErr     error
	replacedDate   timezone.Date
	replacedEntrie []*mealplan.MealPlanEntry
	replaceCalls   int

	deleteErr   error
	deletedDate timezone.Date
	deleteCalls int
}

func (f *fakeRepo) FindByDateRange(_ context.Context, start, end timezone.Date) ([]*mealplan.MealPlanEntry, error) {
	f.gotStart, f.gotEnd = start, end
	return f.rangeResult, f.rangeErr
}

func (f *fakeRepo) ReplaceDay(_ context.Context, date timezone.Date, entries []*mealplan.MealPlanEntry) error {
	f.replaceCalls++
	f.replacedDate = date
	f.replacedEntrie = entries
	return f.replaceErr
}

func (f *fakeRepo) DeleteByDate(_ context.Context, date timezone.Date) error {
	f.deleteCalls++
	f.deletedDate = date
	return f.deleteErr
}

func strptr(s string) *string { return &s }

// TestWeekRange verifies the Monday-Friday bounds for any day in a week,
// independent of which weekday the input falls on.
func TestWeekRange(t *testing.T) {
	t.Parallel()

	// 2026-06-29 is a Monday; the week runs Mon 06-29 .. Fri 07-03.
	wantMonday := timezone.NewDate(2026, time.June, 29)
	wantFriday := timezone.NewDate(2026, time.July, 3)

	cases := []struct {
		name  string
		input timezone.Date
	}{
		{"monday", timezone.NewDate(2026, time.June, 29)},
		{"tuesday", timezone.NewDate(2026, time.June, 30)},
		{"friday", timezone.NewDate(2026, time.July, 3)},
		{"saturday", timezone.NewDate(2026, time.July, 4)},
		{"sunday", timezone.NewDate(2026, time.July, 5)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			monday, friday := WeekRange(tc.input)
			if monday != wantMonday {
				t.Errorf("monday: got %s, want %s", monday, wantMonday)
			}
			if friday != wantFriday {
				t.Errorf("friday: got %s, want %s", friday, wantFriday)
			}
		})
	}
}

// TestGetWeek_PassesMondayFridayRangeToRepo verifies GetWeek normalizes any day
// in the week to the Monday-Friday bounds before querying, and returns the
// repository rows unchanged.
func TestGetWeek_PassesMondayFridayRangeToRepo(t *testing.T) {
	t.Parallel()

	want := []*mealplan.MealPlanEntry{{Dish: "Nudeln"}}
	repo := &fakeRepo{rangeResult: want}
	svc := NewService(repo)

	// A Wednesday — the service must still query Mon..Fri of that week.
	got, err := svc.GetWeek(context.Background(), timezone.NewDate(2026, time.July, 1))
	if err != nil {
		t.Fatalf("GetWeek returned error: %v", err)
	}
	if repo.gotStart != timezone.NewDate(2026, time.June, 29) {
		t.Errorf("start: got %s, want 2026-06-29 (Monday)", repo.gotStart)
	}
	if repo.gotEnd != timezone.NewDate(2026, time.July, 3) {
		t.Errorf("end: got %s, want 2026-07-03 (Friday)", repo.gotEnd)
	}
	if len(got) != 1 || got[0].Dish != "Nudeln" {
		t.Errorf("GetWeek did not pass through repo rows: %+v", got)
	}
}

func TestGetWeek_PropagatesRepoError(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{rangeErr: errors.New("db down")}
	svc := NewService(repo)
	if _, err := svc.GetWeek(context.Background(), timezone.NewDate(2026, time.July, 1)); err == nil {
		t.Fatal("expected error from GetWeek, got nil")
	}
}

// TestSetDay_TrimsAssignsPositionsAndDropsBlanks is the core write-path test:
// blank/whitespace-only dishes are dropped, surviving dishes are trimmed and
// re-numbered from 0, and notes are normalized (trimmed, empty→nil).
func TestSetDay_TrimsAssignsPositionsAndDropsBlanks(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	svc := NewService(repo)
	date := timezone.NewDate(2026, time.July, 1)

	dishes := []DishInput{
		{Dish: "  Menü 1  ", Note: strptr("  vegetarisch ")},
		{Dish: "   ", Note: strptr("dropped")}, // blank dish — dropped entirely
		{Dish: "Menü 2", Note: strptr("   ")},  // whitespace note → nil
		{Dish: "Suppe", Note: nil},
	}

	if err := svc.SetDay(context.Background(), date, dishes); err != nil {
		t.Fatalf("SetDay returned error: %v", err)
	}
	if repo.replaceCalls != 1 {
		t.Fatalf("expected 1 ReplaceDay call, got %d", repo.replaceCalls)
	}
	if repo.replacedDate != date {
		t.Errorf("ReplaceDay date: got %s, want %s", repo.replacedDate, date)
	}

	got := repo.replacedEntrie
	if len(got) != 3 {
		t.Fatalf("expected 3 surviving dishes, got %d: %+v", len(got), got)
	}
	// Position 0: trimmed dish + trimmed note.
	if got[0].Position != 0 || got[0].Dish != "Menü 1" || got[0].Note == nil || *got[0].Note != "vegetarisch" {
		t.Errorf("entry 0 wrong: %+v (note=%v)", got[0], deref(got[0].Note))
	}
	// Position 1: whitespace-only note collapses to nil.
	if got[1].Position != 1 || got[1].Dish != "Menü 2" || got[1].Note != nil {
		t.Errorf("entry 1 wrong: %+v (note=%v)", got[1], deref(got[1].Note))
	}
	// Position 2: nil note stays nil.
	if got[2].Position != 2 || got[2].Dish != "Suppe" || got[2].Note != nil {
		t.Errorf("entry 2 wrong: %+v (note=%v)", got[2], deref(got[2].Note))
	}
}

// TestSetDay_AllBlankClearsDay verifies an all-blank submission still calls
// ReplaceDay with an empty slice (the repo semantics clear the day), never
// persisting a phantom empty dish.
func TestSetDay_AllBlankClearsDay(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	svc := NewService(repo)

	err := svc.SetDay(context.Background(), timezone.NewDate(2026, time.July, 1), []DishInput{
		{Dish: "   "},
		{Dish: ""},
	})
	if err != nil {
		t.Fatalf("SetDay returned error: %v", err)
	}
	if repo.replaceCalls != 1 {
		t.Fatalf("expected ReplaceDay to be called once, got %d", repo.replaceCalls)
	}
	if len(repo.replacedEntrie) != 0 {
		t.Errorf("expected empty entries to clear the day, got %+v", repo.replacedEntrie)
	}
}

func TestSetDay_ZeroDateRejected(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	svc := NewService(repo)
	if err := svc.SetDay(context.Background(), timezone.Date{}, []DishInput{{Dish: "x"}}); err == nil {
		t.Fatal("expected error for zero date, got nil")
	}
	if repo.replaceCalls != 0 {
		t.Errorf("ReplaceDay must not be called for a zero date, got %d calls", repo.replaceCalls)
	}
}

// TestSetDay_WeekendRejected verifies Saturday/Sunday writes fail with
// ErrInvalidMealDate and never reach the repository: a weekend row would be
// invisible (GetWeek only reads Mon-Fri) so it must not be persisted.
func TestSetDay_WeekendRejected(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		date timezone.Date
	}{
		{"saturday", timezone.NewDate(2026, time.July, 4)},
		{"sunday", timezone.NewDate(2026, time.July, 5)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{}
			svc := NewService(repo)
			err := svc.SetDay(context.Background(), tc.date, []DishInput{{Dish: "Nudeln"}})
			if !errors.Is(err, ErrInvalidMealDate) {
				t.Fatalf("expected ErrInvalidMealDate, got %v", err)
			}
			if repo.replaceCalls != 0 {
				t.Errorf("ReplaceDay must not run for a weekend date, got %d calls", repo.replaceCalls)
			}
		})
	}
}

// TestSetDay_WeekdaysAccepted guards the boundary the weekend check must not
// over-reach: Monday and Friday are valid work-week days.
func TestSetDay_WeekdaysAccepted(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		date timezone.Date
	}{
		{"monday", timezone.NewDate(2026, time.June, 29)},
		{"friday", timezone.NewDate(2026, time.July, 3)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{}
			svc := NewService(repo)
			if err := svc.SetDay(context.Background(), tc.date, []DishInput{{Dish: "Nudeln"}}); err != nil {
				t.Fatalf("SetDay rejected a weekday: %v", err)
			}
			if repo.replaceCalls != 1 {
				t.Errorf("expected ReplaceDay to run for a weekday, got %d calls", repo.replaceCalls)
			}
		})
	}
}

func TestSetDay_PropagatesRepoError(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{replaceErr: errors.New("insert failed")}
	svc := NewService(repo)
	err := svc.SetDay(context.Background(), timezone.NewDate(2026, time.July, 1), []DishInput{{Dish: "x"}})
	if err == nil {
		t.Fatal("expected repo error to propagate, got nil")
	}
}

func TestDelete_CallsRepoWithDate(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	svc := NewService(repo)
	date := timezone.NewDate(2026, time.July, 1)
	if err := svc.Delete(context.Background(), date); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if repo.deleteCalls != 1 || repo.deletedDate != date {
		t.Errorf("Delete did not forward date: calls=%d date=%s", repo.deleteCalls, repo.deletedDate)
	}
}

func TestDelete_ZeroDateRejected(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	svc := NewService(repo)
	if err := svc.Delete(context.Background(), timezone.Date{}); err == nil {
		t.Fatal("expected error for zero date, got nil")
	}
	if repo.deleteCalls != 0 {
		t.Errorf("DeleteByDate must not be called for a zero date, got %d calls", repo.deleteCalls)
	}
}

func TestDelete_PropagatesRepoError(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{deleteErr: errors.New("delete failed")}
	svc := NewService(repo)
	if err := svc.Delete(context.Background(), timezone.NewDate(2026, time.July, 1)); err == nil {
		t.Fatal("expected repo error to propagate, got nil")
	}
}

// TestNormalizeNote covers the note-cleanup helper directly for the edge cases
// the SetDay tests exercise indirectly.
func TestNormalizeNote(t *testing.T) {
	t.Parallel()

	if got := strutil.TrimPtrToNil(nil); got != nil {
		t.Errorf("nil note should stay nil, got %q", deref(got))
	}
	if got := strutil.TrimPtrToNil(strptr("   ")); got != nil {
		t.Errorf("whitespace note should collapse to nil, got %q", deref(got))
	}
	if got := strutil.TrimPtrToNil(strptr("  hot  ")); got == nil || *got != "hot" {
		t.Errorf("note should be trimmed, got %v", deref(got))
	}
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}
