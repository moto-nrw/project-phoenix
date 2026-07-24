package active

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingBalanceAdjustmentRepo struct {
	activeModels.StaffBalanceAdjustmentRepository
	events     *[]string
	adjustment *activeModels.StaffBalanceAdjustment
	resets     []*activeModels.StaffBalanceAdjustment
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
	events  *[]string
	balance int
}

func (s *recordingBalanceMonthService) GetClosingBalanceAsOf(context.Context, int64, timezone.Date) (int, error) {
	*s.events = append(*s.events, "as-of")
	return s.balance, nil
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

func TestStaffBalanceAdjustmentService_LocksEveryMutation(t *testing.T) {
	const (
		staffID   = int64(41)
		decidedBy = int64(42)
	)
	effectiveDate := timezone.NewDate(2026, time.July, 7)

	t.Run("create locks before insert", func(t *testing.T) {
		events := []string{}
		repo := &recordingBalanceAdjustmentRepo{events: &events}
		service := newRecordingBalanceAdjustmentService(&events, repo, nil)

		_, err := service.CreateAdjustment(context.Background(), staffID, decidedBy, CreateBalanceAdjustmentRequest{
			Type:          activeModels.BalanceAdjustmentTypePayout,
			MinutesDelta:  -60,
			EffectiveDate: effectiveDate,
			Note:          "Auszahlung",
		})

		require.NoError(t, err)
		assert.Equal(t, []string{"lock", "list", "create"}, events)
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
		assert.Equal(t, []string{"lock", "list", "as-of", "create"}, events)
		assert.Equal(t, -120, adjustment.MinutesDelta)
	})
}

func TestStaffBalanceAdjustmentService_RejectsWritesThatPrecedeReset(t *testing.T) {
	const (
		staffID   = int64(41)
		decidedBy = int64(42)
	)
	resetDate := timezone.TodayDate()
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
		service := newRecordingBalanceAdjustmentService(&events, repo, nil)

		_, err := service.CreateAdjustment(context.Background(), staffID, decidedBy, CreateBalanceAdjustmentRequest{
			Type:          activeModels.BalanceAdjustmentTypeCompTime,
			MinutesDelta:  -60,
			EffectiveDate: resetDate.AddDays(1),
			Note:          "Neue Periode",
		})

		require.NoError(t, err)
		assert.Equal(t, []string{"lock", "list", "create"}, events)
	})
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
