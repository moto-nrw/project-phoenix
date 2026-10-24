package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/feedback"
	"github.com/moto-nrw/project-phoenix/modules/feedback/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/feedback/internal/ports"
)

type seamStore struct {
	ports.Store
	e *recordingEngine
}

func (s seamStore) Create(ctx context.Context, value domain.Entry) (domain.Entry, domain.OperationStats, error) {
	entry, err := s.e.Submit(ctx, feedback.CreateEntry{Value: value.Value, Day: feedback.Date(value.Day.String()), Time: value.Time.Format("15:04:05"), StudentID: value.StudentID, IsMensaFeedback: value.IsMensaFeedback})
	value.ID = entry.ID
	return value, domain.OperationStats{}, err
}

type seamTransaction struct{}

func (seamTransaction) Run(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type seamSettings struct{}

func (seamSettings) FeedbackEnabled(context.Context) (bool, error)      { return true, nil }
func (seamSettings) FeedbackRetentionDays(context.Context) (int, error) { return 90, nil }
func newTestModule(e *recordingEngine) *Service {
	return New(seamStore{e: e}, seamSettings{}, seamTransaction{}, func(o ports.Observation) {
		if o.Err != nil {
			e.ObserveRejection(o.Operation, o.Duration, o.Err)
		}
	}, func() domain.Date { return domain.Date("2026-08-31") })
}

var _ = time.Second
