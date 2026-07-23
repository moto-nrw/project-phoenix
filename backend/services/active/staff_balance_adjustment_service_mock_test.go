package active

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingBalanceAdjustmentRepo struct {
	activeModels.StaffBalanceAdjustmentRepository
	events     *[]string
	adjustment *activeModels.StaffBalanceAdjustment
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
		slog.New(slog.NewTextHandler(io.Discard, nil)),
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
		assert.Equal(t, []string{"lock", "create"}, events)
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
		assert.Equal(t, []string{"lock", "find", "delete"}, events)
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
		assert.Equal(t, []string{"lock", "as-of", "create"}, events)
		assert.Equal(t, -120, adjustment.MinutesDelta)
	})
}
