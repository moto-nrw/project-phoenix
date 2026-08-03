package active

import (
	"context"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Opening balance (#2132) unit tests over the recording harness from
// staff_balance_adjustment_service_mock_test.go.

func TestStaffBalanceAdjustmentService_OpeningAllowsNegativeTarget(t *testing.T) {
	const (
		staffID   = int64(41)
		decidedBy = int64(42)
	)
	events := []string{}
	repo := &recordingBalanceAdjustmentRepo{events: &events}
	month := &recordingBalanceMonthService{events: &events, balance: 0}
	service := newRecordingBalanceAdjustmentService(&events, repo, month)

	effectiveDate := timezone.TodayDate().AddDays(-1)
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
