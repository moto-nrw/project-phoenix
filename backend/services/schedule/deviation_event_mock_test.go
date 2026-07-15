package schedule

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModel "github.com/moto-nrw/project-phoenix/models/audit"
)

// recordingDeviationEventRepo captures Änderungsprotokoll writes for
// in-package unit tests (#1886). Func-field convention: nil = zero-value
// default (record and succeed).
type recordingDeviationEventRepo struct {
	events    []*auditModel.DeviationEvent
	createErr error
}

func (r *recordingDeviationEventRepo) Create(_ context.Context, event *auditModel.DeviationEvent) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.events = append(r.events, event)
	return nil
}

func (r *recordingDeviationEventRepo) ListByRange(_ context.Context, _, _ timezone.Date, _ *int64, _ *string) ([]*auditModel.DeviationEvent, error) {
	return r.events, nil
}

func (r *recordingDeviationEventRepo) DeleteOlderThan(_ context.Context, _ timezone.Date) (int64, error) {
	return 0, nil
}
