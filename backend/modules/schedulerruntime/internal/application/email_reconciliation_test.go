package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type stubEmailReconciler struct {
	err   error
	calls int
}

func (s *stubEmailReconciler) ReconcileEmailDelivery(context.Context) error {
	s.calls++
	return s.err
}

func TestEmailReconciliationTaskReportsTheAlarmAsAFailure(t *testing.T) {
	t.Parallel()
	reconciler := &stubEmailReconciler{err: errors.New("email webhook is disabled at the provider")}
	var failures []string
	s := &Scheduler{emailReconciliation: reconciler}
	s.workerTracer.Failure = func(_ context.Context, operation, outcome string, _ error) {
		failures = append(failures, operation+"/"+outcome)
	}

	s.checkAndRunEmailReconciliation(context.Background(), &ScheduledTask{Name: "email-delivery-reconciliation"})

	assert.Equal(t, 1, reconciler.calls)
	assert.Equal(t, []string{"email-delivery-reconciliation/alarm"}, failures)
}

func TestEmailReconciliationTaskDoesNotOverlap(t *testing.T) {
	t.Parallel()
	reconciler := &stubEmailReconciler{}
	s := &Scheduler{emailReconciliation: reconciler}

	s.checkAndRunEmailReconciliation(context.Background(), &ScheduledTask{Name: "email-delivery-reconciliation", Running: true})

	assert.Zero(t, reconciler.calls)
}
