package active

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Opening balance (#2132) unit tests over the recording harness from
// staff_balance_adjustment_service_mock_test.go.

func TestStaffBalanceAdjustmentService_OpeningAllowsNegativeTarget(t *testing.T) {
	t.Parallel()

	const (
		staffID   = int64(41)
		decidedBy = int64(42)
	)
	events := []string{}
	repo := &recordingBalanceAdjustmentRepo{events: &events}
	month := &recordingBalanceMonthService{events: &events, balance: 0}
	service := newRecordingBalanceAdjustmentService(&events, repo, month)

	effectiveDate := timezone.NewDate(2026, 8, 24).AddDays(-1)
	adjustment, err := service.CreateOpeningBalance(
		context.Background(),
		staffID,
		decidedBy,
		effectiveDate,
		-330, // -5,5 h from the previous system
		"Übernahme aus Altsystem",
	)

	require.NoError(t, err)
	require.NotNil(t, adjustment)
	assert.Equal(t, activeModels.BalanceAdjustmentTypeOpening, adjustment.Type)
	assert.Equal(t, -330, adjustment.MinutesDelta)
	assert.Equal(t, effectiveDate, adjustment.EffectiveDate)
	// The whole point of the opening type: a negative resulting balance must
	// NOT consult the reduction-capacity guard.
	assert.NotContains(t, events, "capacity")
	assert.Contains(t, events, "lock")
	assert.Contains(t, events, "create")
}

func TestStaffBalanceAdjustmentService_OpeningDeltaTargetsEnteredBalance(t *testing.T) {
	t.Parallel()

	const (
		staffID   = int64(41)
		decidedBy = int64(42)
	)
	events := []string{}
	repo := &recordingBalanceAdjustmentRepo{events: &events}
	// Sessions already recorded before the takeover: live balance 90 min.
	month := &recordingBalanceMonthService{events: &events, balance: 90}
	service := newRecordingBalanceAdjustmentService(&events, repo, month)

	adjustment, err := service.CreateOpeningBalance(
		context.Background(),
		staffID,
		decidedBy,
		timezone.TodayDate().AddDays(-1),
		750, // target 12,5 h
		"Übernahme aus Altsystem",
	)

	require.NoError(t, err)
	// delta = target − live balance as of the Stichtag.
	assert.Equal(t, 660, adjustment.MinutesDelta)
}

func TestStaffBalanceAdjustmentService_OpeningRejectsSecondOpening(t *testing.T) {
	t.Parallel()

	const (
		staffID   = int64(41)
		decidedBy = int64(42)
	)
	events := []string{}
	existing := &activeModels.StaffBalanceAdjustment{
		StaffID:       staffID,
		Type:          activeModels.BalanceAdjustmentTypeOpening,
		EffectiveDate: timezone.TodayDate().AddDays(-10),
	}
	repo := &recordingBalanceAdjustmentRepo{events: &events, resets: []*activeModels.StaffBalanceAdjustment{existing}}
	month := &recordingBalanceMonthService{events: &events}
	service := newRecordingBalanceAdjustmentService(&events, repo, month)

	_, err := service.CreateOpeningBalance(
		context.Background(),
		staffID,
		decidedBy,
		timezone.TodayDate().AddDays(-1),
		600,
		"Zweiter Versuch",
	)

	require.ErrorIs(t, err, ErrOpeningAlreadyExists)
	assert.NotContains(t, events, "create")
}

func TestStaffBalanceAdjustmentService_OpeningRejectsOpenDay(t *testing.T) {
	t.Parallel()

	const (
		staffID   = int64(41)
		decidedBy = int64(42)
	)
	for _, effectiveDate := range []timezone.Date{
		timezone.TodayDate(),
		timezone.TodayDate().AddDays(1),
	} {
		events := []string{}
		service := newRecordingBalanceAdjustmentService(
			&events,
			&recordingBalanceAdjustmentRepo{events: &events},
			nil,
		)

		_, err := service.CreateOpeningBalance(
			context.Background(),
			staffID,
			decidedBy,
			effectiveDate,
			600,
			"Übernahme",
		)

		require.ErrorIs(t, err, ErrAdjustmentInvalid)
		assert.Contains(t, err.Error(), "effective_date must be before today")
		assert.Empty(t, events, "an open cutoff must fail before locking or reading")
	}
}

func TestStaffBalanceAdjustmentService_OpeningRejectsOutOfRangeTarget(t *testing.T) {
	t.Parallel()

	const (
		staffID   = int64(41)
		decidedBy = int64(42)
	)
	for _, balanceMinutes := range []int{maxBalanceCarryoverMinutes + 1, -maxBalanceCarryoverMinutes - 1} {
		events := []string{}
		service := newRecordingBalanceAdjustmentService(
			&events,
			&recordingBalanceAdjustmentRepo{events: &events},
			nil,
		)

		_, err := service.CreateOpeningBalance(
			context.Background(),
			staffID,
			decidedBy,
			timezone.TodayDate().AddDays(-1),
			balanceMinutes,
			"Übernahme",
		)

		require.ErrorIs(t, err, ErrAdjustmentInvalid)
		assert.Empty(t, events)
	}
}

// typeFilteringBalanceAdjustmentRepo refines the recording repo: List answers
// per call like the real repository would — no rows for the opening-duplicate
// lookup, the seeded rows for the rebaseline lookup. The shared recording
// repo returns one static list for every List call, which cannot distinguish
// the two guards CreateOpeningBalance runs back to back.
type typeFilteringBalanceAdjustmentRepo struct {
	*recordingBalanceAdjustmentRepo
	listCalls int
}

func (r *typeFilteringBalanceAdjustmentRepo) List(ctx context.Context, options *modelBase.QueryOptions) ([]*activeModels.StaffBalanceAdjustment, error) {
	r.listCalls++
	if r.listCalls == 1 {
		*r.events = append(*r.events, "list")
		return nil, nil
	}
	return r.recordingBalanceAdjustmentRepo.List(ctx, options)
}

func TestStaffBalanceAdjustmentService_OpeningRejectsExistingLaterReset(t *testing.T) {
	t.Parallel()

	const (
		staffID   = int64(41)
		decidedBy = int64(42)
	)
	events := []string{}
	laterReset := &activeModels.StaffBalanceAdjustment{
		StaffID:       staffID,
		Type:          activeModels.BalanceAdjustmentTypeReset,
		EffectiveDate: timezone.TodayDate().AddDays(-2),
	}
	repo := &typeFilteringBalanceAdjustmentRepo{
		recordingBalanceAdjustmentRepo: &recordingBalanceAdjustmentRepo{events: &events, resets: []*activeModels.StaffBalanceAdjustment{laterReset}},
	}
	month := &recordingBalanceMonthService{events: &events}
	service := NewStaffBalanceAdjustmentService(
		repo,
		month,
		&wtmMockSettings{accountStart: "2026-06-01"},
		slog.New(slog.DiscardHandler),
	)

	_, err := service.CreateOpeningBalance(
		context.Background(),
		staffID,
		decidedBy,
		timezone.TodayDate().AddDays(-5),
		600,
		"Übernahme",
	)

	// No opening exists yet, but a reset on a later date must still trip the
	// dependent-booking guard.
	require.ErrorIs(t, err, ErrAdjustmentHasDependentReset)
	assert.NotContains(t, events, "create")
}

// failingBalanceAdjustmentRepo injects repository failures into the guard
// chain of an opening booking.
type failingBalanceAdjustmentRepo struct {
	*recordingBalanceAdjustmentRepo
	listErr   error
	createErr error
}

func (r *failingBalanceAdjustmentRepo) List(ctx context.Context, options *modelBase.QueryOptions) ([]*activeModels.StaffBalanceAdjustment, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.recordingBalanceAdjustmentRepo.List(ctx, options)
}

func (r *failingBalanceAdjustmentRepo) Create(ctx context.Context, adjustment *activeModels.StaffBalanceAdjustment) error {
	if r.createErr != nil {
		*r.events = append(*r.events, "create")
		return r.createErr
	}
	return r.recordingBalanceAdjustmentRepo.Create(ctx, adjustment)
}

// failingBalanceMonthService fails the as-of balance read the opening delta
// is computed from.
type failingBalanceMonthService struct {
	*recordingBalanceMonthService
	err error
}

func (s *failingBalanceMonthService) GetClosingBalanceAsOf(ctx context.Context, staffID int64, date timezone.Date) (int, error) {
	if s.err != nil {
		return 0, s.err
	}
	return s.recordingBalanceMonthService.GetClosingBalanceAsOf(ctx, staffID, date)
}

func newOpeningTestService(events *[]string, repo activeModels.StaffBalanceAdjustmentRepository, month WorkTimeMonthService) StaffBalanceAdjustmentService {
	return NewStaffBalanceAdjustmentService(
		repo,
		month,
		&wtmMockSettings{accountStart: "2026-06-01"},
		slog.New(slog.DiscardHandler),
	)
}

func TestStaffBalanceAdjustmentService_OpeningRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	const (
		staffID   = int64(41)
		decidedBy = int64(42)
	)
	yesterday := timezone.TodayDate().AddDays(-1)

	tests := []struct {
		name          string
		staffID       int64
		decidedBy     int64
		effectiveDate timezone.Date
		minutes       int
		note          string
		wantMsg       string
	}{
		{
			name: "missing staff", staffID: 0, decidedBy: decidedBy,
			effectiveDate: yesterday, minutes: 600, note: "Übernahme",
			wantMsg: "staff id is required",
		},
		{
			name: "missing actor", staffID: staffID, decidedBy: 0,
			effectiveDate: yesterday, minutes: 600, note: "Übernahme",
			wantMsg: "decided_by is required",
		},
		{
			name: "missing Stichtag", staffID: staffID, decidedBy: decidedBy,
			effectiveDate: timezone.Date(""), minutes: 600, note: "Übernahme",
			wantMsg: "effective_date is required",
		},
		{
			name: "missing Begründung", staffID: staffID, decidedBy: decidedBy,
			effectiveDate: yesterday, minutes: 600, note: "   ",
			wantMsg: "note is required",
		},
		{
			// The Stichtag needs a closed day, or the stored delta goes stale
			// as the day's sessions keep coming in.
			name: "Stichtag today", staffID: staffID, decidedBy: decidedBy,
			effectiveDate: timezone.TodayDate(), minutes: 600, note: "Übernahme",
			wantMsg: "must be before today",
		},
		{
			name: "target above the carryover bound", staffID: staffID, decidedBy: decidedBy,
			effectiveDate: yesterday, minutes: maxBalanceCarryoverMinutes + 1, note: "Übernahme",
			wantMsg: "balance_minutes must be between",
		},
		{
			name: "target below the carryover bound", staffID: staffID, decidedBy: decidedBy,
			effectiveDate: yesterday, minutes: -maxBalanceCarryoverMinutes - 1, note: "Übernahme",
			wantMsg: "balance_minutes must be between",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			events := []string{}
			repo := &recordingBalanceAdjustmentRepo{events: &events}
			service := newOpeningTestService(&events, repo, &recordingBalanceMonthService{events: &events})

			_, err := service.CreateOpeningBalance(context.Background(), tc.staffID, tc.decidedBy, tc.effectiveDate, tc.minutes, tc.note)

			require.ErrorIs(t, err, ErrAdjustmentInvalid)
			assert.Contains(t, err.Error(), tc.wantMsg)
			assert.NotContains(t, events, "create")
		})
	}
}

func TestStaffBalanceAdjustmentService_OpeningRejectsDateBeforeAccountStart(t *testing.T) {
	t.Parallel()

	events := []string{}
	repo := &recordingBalanceAdjustmentRepo{events: &events}
	service := newOpeningTestService(&events, repo, &recordingBalanceMonthService{events: &events})

	// Account start is 2026-06-01; a Stichtag before it would sit outside
	// every carry chain and silently never move the Stundenkonto.
	_, err := service.CreateOpeningBalance(
		context.Background(), 41, 42,
		timezone.NewDate(2026, time.May, 31), 600, "Übernahme",
	)

	require.Error(t, err)
	assert.NotContains(t, events, "create")
}

func TestStaffBalanceAdjustmentService_OpeningPropagatesRepositoryFailures(t *testing.T) {
	t.Parallel()

	boom := errors.New("db down")
	yesterday := timezone.TodayDate().AddDays(-1)

	t.Run("existing-booking lookup", func(t *testing.T) {
		events := []string{}
		repo := &failingBalanceAdjustmentRepo{
			recordingBalanceAdjustmentRepo: &recordingBalanceAdjustmentRepo{events: &events},
			listErr:                        boom,
		}
		service := newOpeningTestService(&events, repo, &recordingBalanceMonthService{events: &events})

		_, err := service.CreateOpeningBalance(context.Background(), 41, 42, yesterday, 600, "Übernahme")

		require.Error(t, err)
		assert.NotContains(t, events, "create")
	})

	t.Run("as-of balance", func(t *testing.T) {
		events := []string{}
		repo := &recordingBalanceAdjustmentRepo{events: &events}
		month := &failingBalanceMonthService{
			recordingBalanceMonthService: &recordingBalanceMonthService{events: &events},
			err:                          boom,
		}
		service := newOpeningTestService(&events, repo, month)

		_, err := service.CreateOpeningBalance(context.Background(), 41, 42, yesterday, 600, "Übernahme")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to compute balance for opening")
	})

	t.Run("insert", func(t *testing.T) {
		events := []string{}
		repo := &failingBalanceAdjustmentRepo{
			recordingBalanceAdjustmentRepo: &recordingBalanceAdjustmentRepo{events: &events},
			createErr:                      boom,
		}
		service := newOpeningTestService(&events, repo, &recordingBalanceMonthService{events: &events})

		_, err := service.CreateOpeningBalance(context.Background(), 41, 42, yesterday, 600, "Übernahme")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create opening balance")
	})
}

// TestStaffBalanceAdjustmentService_ValidateOpeningTakesNoLock pins the
// contract the bulk import relies on: the preview runs the same guards but
// must not take the per-staff lock, or concurrent imports could deadlock on
// file order.
func TestStaffBalanceAdjustmentService_ValidateOpeningTakesNoLock(t *testing.T) {
	t.Parallel()

	events := []string{}
	repo := &recordingBalanceAdjustmentRepo{events: &events}
	service := newOpeningTestService(&events, repo, &recordingBalanceMonthService{events: &events})

	err := service.ValidateOpeningBalance(
		context.Background(), 41, 42,
		timezone.TodayDate().AddDays(-1), -330, "Übernahme",
	)

	require.NoError(t, err)
	assert.NotContains(t, events, "lock")
	assert.NotContains(t, events, "create", "validation must not book anything")
}

func TestStaffBalanceAdjustmentService_ValidateOpeningReportsTheSameRejection(t *testing.T) {
	t.Parallel()

	events := []string{}
	existing := &activeModels.StaffBalanceAdjustment{
		StaffID:       41,
		Type:          activeModels.BalanceAdjustmentTypeOpening,
		EffectiveDate: timezone.TodayDate().AddDays(-30),
	}
	repo := &recordingBalanceAdjustmentRepo{events: &events, resets: []*activeModels.StaffBalanceAdjustment{existing}}
	service := newOpeningTestService(&events, repo, &recordingBalanceMonthService{events: &events})

	err := service.ValidateOpeningBalance(
		context.Background(), 41, 42,
		timezone.TodayDate().AddDays(-1), 600, "Übernahme",
	)

	require.ErrorIs(t, err, ErrOpeningAlreadyExists)
}
