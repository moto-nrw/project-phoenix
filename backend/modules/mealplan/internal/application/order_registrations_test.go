package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failedParticipationBatch struct {
	ports.Store
	batchErr error
	dailyErr error
}

func (s failedParticipationBatch) FindParticipationDays(context.Context, []int64, map[domain.Date]time.Time) (map[domain.Date]map[int64]domain.DailyParticipation, domain.OperationStats, error) {
	return nil, domain.OperationStats{}, s.batchErr
}

func (s failedParticipationBatch) FindDailyParticipation(context.Context, []int64, domain.Date, time.Time) (map[int64]domain.DailyParticipation, domain.OperationStats, error) {
	return map[int64]domain.DailyParticipation{}, domain.OperationStats{}, s.dailyErr
}

func TestOrderBatchFailureKeepsDailyErrorPriority(t *testing.T) {
	t.Parallel()
	monday := domain.Date("2026-09-07")
	batchErr := errors.New("later participation unavailable")
	rosterErr := errors.New("Tuesday roster unavailable")
	dailyErr := errors.New("Monday participation unavailable")
	for _, tc := range []struct {
		name  string
		daily error
		want  error
	}{
		{"Monday succeeds before Tuesday roster fails", nil, rosterErr},
		{"Monday participation fails first", dailyErr, dailyErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var rosterReads []string
			directory := NewParticipantDirectory(func(_ context.Context, date string) ([]mealplan.ParticipantCandidate, error) {
				rosterReads = append(rosterReads, date)
				if date != monday.String() {
					return nil, rosterErr
				}
				return []mealplan.ParticipantCandidate{{StudentID: 1}}, nil
			}, nil)
			service := &Service{store: failedParticipationBatch{batchErr: batchErr, dailyErr: tc.daily}, directory: directory}
			rules := defaultOrderingRules("09:00")
			seen := registrations{}
			prepareOrderRegistrations(seen, rules, domain.CalendarDays{}, monday, monday.AddDays(4), monday.BerlinMidnight())
			_, _, err := service.childrenOn(context.Background(), rules, monday, rules.AbsenceCutoff(monday), domain.CalendarDays{}, seen)
			if err == nil {
				_, _, err = service.childrenOn(context.Background(), rules, monday.AddDays(1), rules.AbsenceCutoff(monday.AddDays(1)), domain.CalendarDays{}, seen)
			}
			require.ErrorIs(t, err, tc.want)
			assert.NotErrorIs(t, err, batchErr)
			if tc.daily != nil {
				assert.Equal(t, []string{monday.String()}, rosterReads)
			}
		})
	}
}
