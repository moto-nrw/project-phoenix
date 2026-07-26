package active

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingBalanceAdjustmentRepo struct {
	activeModels.StaffBalanceAdjustmentRepository
	events     *[]string
	adjustment *activeModels.StaffBalanceAdjustment
	resets     []*activeModels.StaffBalanceAdjustment
	findErr    error
}

func (r *recordingBalanceAdjustmentRepo) LockStaffBalanceWrites(context.Context, int64) error {
	*r.events = append(*r.events, "lock")
	return nil
}

func (r *recordingBalanceAdjustmentRepo) Create(_ context.Context, adjustment *activeModels.StaffBalanceAdjustment) error {
	*r.events = append(*r.events, "create")
	r.adjustment = adjustment
	return nil
}

func (r *recordingBalanceAdjustmentRepo) FindByID(context.Context, any) (*activeModels.StaffBalanceAdjustment, error) {
	*r.events = append(*r.events, "find")
	if r.findErr != nil {
		return nil, r.findErr
	}
	return r.adjustment, nil
}

func (r *recordingBalanceAdjustmentRepo) Delete(context.Context, any) error {
	*r.events = append(*r.events, "delete")
	return nil
}

func (r *recordingBalanceAdjustmentRepo) List(context.Context, *modelBase.QueryOptions) ([]*activeModels.StaffBalanceAdjustment, error) {
	*r.events = append(*r.events, "list")
	return r.resets, nil
}

type recordingBalanceMonthService struct {
	WorkTimeMonthService
	events    *[]string
	balance   int
	capacity  *int
	effective timezone.Date
}

func (s *recordingBalanceMonthService) GetClosingBalanceAsOf(context.Context, int64, timezone.Date) (int, error) {
	*s.events = append(*s.events, "as-of")
	return s.balance, nil
}

func (s *recordingBalanceMonthService) GetBalanceReductionCapacity(_ context.Context, _ int64, effectiveDate timezone.Date) (int, error) {
	*s.events = append(*s.events, "capacity")
	s.effective = effectiveDate
	if s.capacity != nil {
		return *s.capacity, nil
	}
	return s.balance, nil
}

type recordingAdjustmentFreezeReader struct {
	events   *[]string
	snapshot *activeModels.StaffMonthBalanceSnapshot
}

func (r *recordingAdjustmentFreezeReader) GetLatestClosedThrough(context.Context, int64, int, int) (*activeModels.StaffMonthBalanceSnapshot, error) {
	*r.events = append(*r.events, "snapshot")
	return r.snapshot, nil
}

func newRecordingBalanceAdjustmentService(
	events *[]string,
	repo *recordingBalanceAdjustmentRepo,
	monthService WorkTimeMonthService,
) StaffBalanceAdjustmentService {
	return NewStaffBalanceAdjustmentService(
		repo,
		monthService,
		&wtmMockSettings{accountStart: "2026-06-01"},
		slog.New(slog.DiscardHandler),
	)
}

func TestStaffBalanceAdjustmentService_BroadcastsAfterCommit(t *testing.T) {
	events := []string{}
	repo := &recordingBalanceAdjustmentRepo{events: &events}
	monthService := &recordingBalanceMonthService{
		events:  &events,
		balance: 600,
	}
	service := newRecordingBalanceAdjustmentService(&events, repo, monthService)
	broadcaster := testpkg.NewRecordingBroadcaster()
	service.(interface {
		SetBroadcaster(realtime.Broadcaster)
	}).SetBroadcaster(broadcaster)
	ctx, commit := tenant.WithAfterCommitHooksForTest(
		tenant.WithTenantID(context.Background(), 42),
	)

	_, err := service.CreateAdjustment(ctx, 41, 42, CreateBalanceAdjustmentRequest{
		Type:          activeModels.BalanceAdjustmentTypePayout,
		MinutesDelta:  -60,
		EffectiveDate: timezone.NewDate(2026, time.July, 7),
		Note:          "Auszahlung",
	})
	require.NoError(t, err)
	assert.Empty(t, broadcaster.Events())

	commit()

	broadcastEvents := broadcaster.EventsOfType(realtime.EventStaffTimeTrackingChanged)
	require.Len(t, broadcastEvents, 1)
	calls := broadcaster.CallsByMethod("tenant")
	require.Len(t, calls, 1)
	assert.Equal(t, int64(42), calls[0].TenantID)
}

func TestStaffBalanceAdjustmentService_ChecksFrozenMonthAfterLock(t *testing.T) {
	const (
		staffID   = int64(41)
		decidedBy = int64(42)
	)
	effectiveDate := timezone.NewDate(2026, time.July, 7)
	frozen := &activeModels.StaffMonthBalanceSnapshot{
		StaffID: staffID,
		Year:    effectiveDate.Year,
		Month:   int(effectiveDate.Month),
	}

	for _, tc := range []struct {
		name string
		run  func(StaffBalanceAdjustmentService) error
	}{
		{
			name: "create",
			run: func(service StaffBalanceAdjustmentService) error {
				_, err := service.CreateAdjustment(context.Background(), staffID, decidedBy, CreateBalanceAdjustmentRequest{
					Type:          activeModels.BalanceAdjustmentTypePayout,
					MinutesDelta:  -60,
					EffectiveDate: effectiveDate,
					Note:          "Auszahlung",
				})
				return err
			},
		},
		{
			name: "reset",
			run: func(service StaffBalanceAdjustmentService) error {
				_, err := service.ResetBalance(
					context.Background(),
					staffID,
					decidedBy,
					effectiveDate,
					60,
					"Schuljahreswechsel",
				)
				return err
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := []string{}
			repo := &recordingBalanceAdjustmentRepo{events: &events}
			service := newRecordingBalanceAdjustmentService(&events, repo, nil)
			service.SetSnapshotReader(&recordingAdjustmentFreezeReader{
				events:   &events,
				snapshot: frozen,
			})

			err := tc.run(service)

			require.ErrorIs(t, err, ErrAdjustmentInvalid)
			assert.Equal(t, []string{"lock", "snapshot"}, events)
		})
	}
}

func TestStaffBalanceAdjustmentService_LocksEveryMutation(t *testing.T) {
	const (
		staffID   = int64(41)
		decidedBy = int64(42)
	)
	effectiveDate := timezone.NewDate(2026, time.July, 7)

	t.Run("create locks before insert", func(t *testing.T) {
		events := []string{}
		repo := &recordingBalanceAdjustmentRepo{events: &events}
		monthService := &recordingBalanceMonthService{events: &events, balance: 60}
		service := newRecordingBalanceAdjustmentService(&events, repo, monthService)

		_, err := service.CreateAdjustment(context.Background(), staffID, decidedBy, CreateBalanceAdjustmentRequest{
			Type:          activeModels.BalanceAdjustmentTypePayout,
			MinutesDelta:  -60,
			EffectiveDate: effectiveDate,
			Note:          "Auszahlung",
		})

		require.NoError(t, err)
		assert.Equal(t, []string{"lock", "list", "capacity", "create"}, events)
	})

	t.Run("delete locks before lookup and delete", func(t *testing.T) {
		events := []string{}
		repo := &recordingBalanceAdjustmentRepo{
			events: &events,
			adjustment: &activeModels.StaffBalanceAdjustment{
				StaffID: staffID,
			},
		}
		service := newRecordingBalanceAdjustmentService(&events, repo, nil)

		err := service.DeleteAdjustment(context.Background(), staffID, 99)

		require.NoError(t, err)
		assert.Equal(t, []string{"lock", "find", "list", "delete"}, events)
	})

	t.Run("reset locks the as-of read and insert", func(t *testing.T) {
		events := []string{}
		repo := &recordingBalanceAdjustmentRepo{events: &events}
		monthService := &recordingBalanceMonthService{
			events:  &events,
			balance: 180,
		}
		service := newRecordingBalanceAdjustmentService(&events, repo, monthService)

		adjustment, err := service.ResetBalance(
			context.Background(),
			staffID,
			decidedBy,
			effectiveDate,
			60,
			"Schuljahreswechsel",
		)

		require.NoError(t, err)
		assert.Equal(t, []string{"lock", "list", "as-of", "capacity", "create"}, events)
		assert.Equal(t, -120, adjustment.MinutesDelta)
	})
}

func TestStaffBalanceAdjustmentService_RejectsWritesThatPrecedeReset(t *testing.T) {
	const (
		staffID   = int64(41)
		decidedBy = int64(42)
	)
	resetDate := timezone.TodayDate().AddDays(-1)
	reset := &activeModels.StaffBalanceAdjustment{
		StaffID:       staffID,
		Type:          activeModels.BalanceAdjustmentTypeReset,
		EffectiveDate: resetDate,
	}
	reset.ID = 20

	t.Run("adjustment on reset date", func(t *testing.T) {
		events := []string{}
		repo := &recordingBalanceAdjustmentRepo{
			events: &events,
			resets: []*activeModels.StaffBalanceAdjustment{reset},
		}
		service := newRecordingBalanceAdjustmentService(&events, repo, nil)

		_, err := service.CreateAdjustment(context.Background(), staffID, decidedBy, CreateBalanceAdjustmentRequest{
			Type:          activeModels.BalanceAdjustmentTypePayout,
			MinutesDelta:  -60,
			EffectiveDate: resetDate,
			Note:          "Nachträgliche Auszahlung",
		})

		require.ErrorIs(t, err, ErrAdjustmentHasDependentReset)
		assert.Equal(t, []string{"lock", "list"}, events)
	})

	t.Run("reset before later reset", func(t *testing.T) {
		events := []string{}
		repo := &recordingBalanceAdjustmentRepo{
			events: &events,
			resets: []*activeModels.StaffBalanceAdjustment{reset},
		}
		monthService := &recordingBalanceMonthService{events: &events, balance: 180}
		service := newRecordingBalanceAdjustmentService(&events, repo, monthService)

		_, err := service.ResetBalance(
			context.Background(),
			staffID,
			decidedBy,
			resetDate.AddDays(-1),
			0,
			"Früherer Reset",
		)

		require.ErrorIs(t, err, ErrAdjustmentHasDependentReset)
		assert.Equal(t, []string{"lock", "list"}, events)
	})

	t.Run("duplicate reset date", func(t *testing.T) {
		events := []string{}
		repo := &recordingBalanceAdjustmentRepo{
			events: &events,
			resets: []*activeModels.StaffBalanceAdjustment{reset},
		}
		monthService := &recordingBalanceMonthService{events: &events, balance: 180}
		service := newRecordingBalanceAdjustmentService(&events, repo, monthService)

		_, err := service.ResetBalance(
			context.Background(),
			staffID,
			decidedBy,
			resetDate,
			0,
			"Doppelter Reset",
		)

		require.ErrorIs(t, err, ErrBalanceAlreadyReset)
		assert.Equal(t, []string{"lock", "list"}, events)
	})

	t.Run("adjustment after reset", func(t *testing.T) {
		events := []string{}
		repo := &recordingBalanceAdjustmentRepo{events: &events}
		monthService := &recordingBalanceMonthService{events: &events, balance: 60}
		service := newRecordingBalanceAdjustmentService(&events, repo, monthService)

		_, err := service.CreateAdjustment(context.Background(), staffID, decidedBy, CreateBalanceAdjustmentRequest{
			Type:          activeModels.BalanceAdjustmentTypeCompTime,
			MinutesDelta:  -60,
			EffectiveDate: resetDate.AddDays(1),
			Note:          "Neue Periode",
		})

		require.NoError(t, err)
		assert.Equal(t, []string{"lock", "list", "capacity", "create"}, events)
	})
}

func TestStaffBalanceAdjustmentService_RejectsAdjustmentAboveClosingBalance(t *testing.T) {
	const (
		staffID   = int64(41)
		decidedBy = int64(42)
	)
	effectiveDate := timezone.NewDate(2026, time.July, 7)

	for _, adjustmentType := range []string{
		activeModels.BalanceAdjustmentTypePayout,
		activeModels.BalanceAdjustmentTypeCompTime,
	} {
		t.Run(adjustmentType, func(t *testing.T) {
			events := []string{}
			repo := &recordingBalanceAdjustmentRepo{events: &events}
			monthService := &recordingBalanceMonthService{events: &events, balance: 59}
			service := newRecordingBalanceAdjustmentService(&events, repo, monthService)

			_, err := service.CreateAdjustment(context.Background(), staffID, decidedBy, CreateBalanceAdjustmentRequest{
				Type:          adjustmentType,
				MinutesDelta:  -60,
				EffectiveDate: effectiveDate,
				Note:          "Nicht gedeckter Abzug",
			})

			require.ErrorIs(t, err, ErrAdjustmentExceedsBalance)
			assert.Contains(t, err.Error(), "available capacity of 59 minutes")
			assert.Equal(t, []string{"lock", "list", "capacity"}, events)
		})
	}
}

func TestStaffBalanceAdjustmentService_UsesTimelineReductionCapacity(t *testing.T) {
	const (
		staffID   = int64(41)
		decidedBy = int64(42)
	)
	effectiveDate := timezone.NewDate(2026, time.July, 7)
	capacity := 30
	events := []string{}
	repo := &recordingBalanceAdjustmentRepo{events: &events}
	monthService := &recordingBalanceMonthService{
		events:   &events,
		balance:  300,
		capacity: &capacity,
	}
	service := newRecordingBalanceAdjustmentService(&events, repo, monthService)

	_, err := service.CreateAdjustment(context.Background(), staffID, decidedBy, CreateBalanceAdjustmentRequest{
		Type:          activeModels.BalanceAdjustmentTypePayout,
		MinutesDelta:  -60,
		EffectiveDate: effectiveDate,
		Note:          "Nicht gedeckter Abzug",
	})

	require.ErrorIs(t, err, ErrAdjustmentExceedsBalance)
	assert.Equal(t, effectiveDate, monthService.effective)
	assert.Equal(t, []string{"lock", "list", "capacity"}, events)
}

func TestStaffBalanceAdjustmentService_RejectsResetThatInvalidatesFutureReduction(t *testing.T) {
	const (
		staffID   = int64(41)
		decidedBy = int64(42)
	)
	effectiveDate := timezone.NewDate(2026, time.July, 7)
	capacity := 119
	events := []string{}
	repo := &recordingBalanceAdjustmentRepo{events: &events}
	monthService := &recordingBalanceMonthService{
		events:   &events,
		balance:  120,
		capacity: &capacity,
	}
	service := newRecordingBalanceAdjustmentService(&events, repo, monthService)

	_, err := service.ResetBalance(
		context.Background(),
		staffID,
		decidedBy,
		effectiveDate,
		0,
		"Schuljahreswechsel",
	)

	require.ErrorIs(t, err, ErrAdjustmentExceedsBalance)
	assert.Contains(t, err.Error(), "reset reduction of 120 minutes exceeds available capacity of 119 minutes")
	assert.Equal(t, effectiveDate, monthService.effective)
	assert.Equal(t, []string{"lock", "list", "as-of", "capacity"}, events)
	assert.Nil(t, repo.adjustment)
}

func TestStaffBalanceAdjustmentService_RejectsOutOfRangeAmounts(t *testing.T) {
	const (
		staffID   = int64(41)
		decidedBy = int64(42)
	)
	effectiveDate := timezone.NewDate(2026, time.July, 7)

	tests := []struct {
		name string
		call func(StaffBalanceAdjustmentService) error
	}{
		{
			name: "payout above UI limit",
			call: func(service StaffBalanceAdjustmentService) error {
				_, err := service.CreateAdjustment(context.Background(), staffID, decidedBy, CreateBalanceAdjustmentRequest{
					Type:          activeModels.BalanceAdjustmentTypePayout,
					MinutesDelta:  -maxBalanceAdjustmentMinutes - 1,
					EffectiveDate: effectiveDate,
					Note:          "Auszahlung",
				})
				return err
			},
		},
		{
			name: "negative reset carryover",
			call: func(service StaffBalanceAdjustmentService) error {
				_, err := service.ResetBalance(
					context.Background(),
					staffID,
					decidedBy,
					effectiveDate,
					-1,
					"Schuljahreswechsel",
				)
				return err
			},
		},
		{
			name: "reset carryover above UI limit",
			call: func(service StaffBalanceAdjustmentService) error {
				_, err := service.ResetBalance(
					context.Background(),
					staffID,
					decidedBy,
					effectiveDate,
					maxBalanceCarryoverMinutes+1,
					"Schuljahreswechsel",
				)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := []string{}
			service := newRecordingBalanceAdjustmentService(
				&events,
				&recordingBalanceAdjustmentRepo{events: &events},
				nil,
			)

			err := tt.call(service)

			require.ErrorIs(t, err, ErrAdjustmentInvalid)
			assert.Empty(t, events, "invalid input must fail before locking or reading")
		})
	}
}

func TestStaffBalanceAdjustmentService_RejectsResetOnOpenDay(t *testing.T) {
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

		_, err := service.ResetBalance(
			context.Background(),
			staffID,
			decidedBy,
			effectiveDate,
			0,
			"Schuljahreswechsel",
		)

		require.ErrorIs(t, err, ErrAdjustmentInvalid)
		assert.Contains(t, err.Error(), "effective_date must be before today")
		assert.Empty(t, events, "an open cutoff must fail before locking or reading")
	}
}

func TestStaffBalanceAdjustmentService_BlocksDeleteWhenLaterResetDependsOnAdjustment(t *testing.T) {
	const staffID = int64(41)
	events := []string{}
	effectiveDate := timezone.NewDate(2026, time.July, 1)
	adjustment := &activeModels.StaffBalanceAdjustment{
		StaffID:       staffID,
		Type:          activeModels.BalanceAdjustmentTypeReset,
		EffectiveDate: effectiveDate,
	}
	adjustment.ID = 10
	laterReset := &activeModels.StaffBalanceAdjustment{
		StaffID:       staffID,
		Type:          activeModels.BalanceAdjustmentTypeReset,
		EffectiveDate: effectiveDate.AddDays(30),
	}
	laterReset.ID = 20
	repo := &recordingBalanceAdjustmentRepo{
		events:     &events,
		adjustment: adjustment,
		resets:     []*activeModels.StaffBalanceAdjustment{adjustment, laterReset},
	}
	service := newRecordingBalanceAdjustmentService(&events, repo, nil)

	err := service.DeleteAdjustment(context.Background(), staffID, adjustment.ID)

	require.ErrorIs(t, err, ErrAdjustmentHasDependentReset)
	assert.Equal(t, []string{"lock", "find", "list"}, events)
}

func TestStaffBalanceAdjustmentService_BlocksPositiveResetDeletionWhenLaterDebitsDependOnIt(t *testing.T) {
	const staffID = int64(41)
	events := []string{}
	effectiveDate := timezone.NewDate(2026, time.July, 1)
	reset := &activeModels.StaffBalanceAdjustment{
		StaffID:       staffID,
		Type:          activeModels.BalanceAdjustmentTypeReset,
		MinutesDelta:  120,
		EffectiveDate: effectiveDate,
	}
	reset.ID = 10
	repo := &recordingBalanceAdjustmentRepo{
		events:     &events,
		adjustment: reset,
		resets:     []*activeModels.StaffBalanceAdjustment{reset},
	}
	capacity := 60
	monthService := &recordingBalanceMonthService{
		events:   &events,
		capacity: &capacity,
	}
	service := newRecordingBalanceAdjustmentService(&events, repo, monthService)

	err := service.DeleteAdjustment(context.Background(), staffID, reset.ID)

	require.ErrorIs(t, err, ErrAdjustmentExceedsBalance)
	assert.Contains(t, err.Error(), "would leave capacity of -60 minutes")
	assert.Equal(t, effectiveDate, monthService.effective)
	assert.Equal(t, []string{"lock", "find", "list", "capacity"}, events)
}

// Far-future effective dates have no defined closing balance (the carry
// chain is bounded); they must fail as invalid input, not surface as a 500
// from the balance read (#1420 review).
func TestStaffBalanceAdjustmentService_RejectsFarFutureEffectiveDate(t *testing.T) {
	events := []string{}
	service := newRecordingBalanceAdjustmentService(
		&events,
		&recordingBalanceAdjustmentRepo{events: &events},
		nil,
	)

	_, err := service.CreateAdjustment(context.Background(), int64(41), int64(42), CreateBalanceAdjustmentRequest{
		Type:          activeModels.BalanceAdjustmentTypePayout,
		MinutesDelta:  -60,
		EffectiveDate: timezone.TodayDate().AddDays(420),
		Note:          "Auszahlung",
	})

	require.ErrorIs(t, err, ErrAdjustmentInvalid)
	assert.Contains(t, err.Error(), "months ahead")
	assert.Empty(t, events, "invalid input must fail before locking or reading")
}

// Only a genuine missing row is a 404: database or transaction failures from
// the delete lookup must propagate, not masquerade as ErrAdjustmentNotFound
// (#1420 review).
func TestStaffBalanceAdjustmentService_DeletePropagatesLookupFailures(t *testing.T) {
	t.Run("no rows maps to not found", func(t *testing.T) {
		events := []string{}
		repo := &recordingBalanceAdjustmentRepo{events: &events, findErr: sql.ErrNoRows}
		service := newRecordingBalanceAdjustmentService(&events, repo, nil)

		err := service.DeleteAdjustment(context.Background(), int64(41), 99)

		require.ErrorIs(t, err, ErrAdjustmentNotFound)
	})

	t.Run("database failure stays a database failure", func(t *testing.T) {
		events := []string{}
		lookupErr := errors.New("connection reset")
		repo := &recordingBalanceAdjustmentRepo{events: &events, findErr: lookupErr}
		service := newRecordingBalanceAdjustmentService(&events, repo, nil)

		err := service.DeleteAdjustment(context.Background(), int64(41), 99)

		require.Error(t, err)
		require.NotErrorIs(t, err, ErrAdjustmentNotFound)
		require.ErrorIs(t, err, lookupErr)
		assert.Contains(t, err.Error(), "failed to load balance adjustment")
	})
}
