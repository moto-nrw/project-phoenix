package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type statusAuditCall struct {
	studentID int64
	before    string
	after     string
}

type fakeStudentLifecycleAuditor struct {
	calls []statusAuditCall
	err   error
}

func (f *fakeStudentLifecycleAuditor) RecordSystemStatusChange(
	_ context.Context,
	studentID int64,
	before string,
	after string,
) error {
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, statusAuditCall{studentID: studentID, before: before, after: after})
	return nil
}

func TestRunActivateStudentsForTenant_AuditsSystemTransition(t *testing.T) {
	t.Parallel()

	const studentID int64 = 701
	repo := &fakeStudentLifecycleRepo{pendingDue: []int64{studentID}}
	auditor := &fakeStudentLifecycleAuditor{}
	s := unitScheduler(&Scheduler{
		studentLifecycleRepo:  repo,
		studentLifecycleAudit: auditor})

	err := s.runActivateStudentsForTenantWithError(context.Background(), 7, time.Now())

	require.NoError(t, err)
	require.Len(t, auditor.calls, 1)
	assert.Equal(t, int64(701), auditor.calls[0].studentID)
	assert.Equal(t, studentStatusPending, auditor.calls[0].before)
	assert.Equal(t, studentStatusActive, auditor.calls[0].after)
}

func TestRunActivateStudentsForTenant_PropagatesAuditFailure(t *testing.T) {
	t.Parallel()

	const studentID int64 = 702
	repo := &fakeStudentLifecycleRepo{activeDue: []int64{studentID}}
	auditErr := errors.New("audit unavailable")
	s := unitScheduler(&Scheduler{
		studentLifecycleRepo: repo,
		studentLifecycleAudit: &fakeStudentLifecycleAuditor{
			err: auditErr,
		}})

	err := s.runActivateStudentsForTenantWithError(context.Background(), 7, time.Now())

	require.ErrorIs(t, err, auditErr)
}

func TestRunActivateStudentsForTenant_SkipsStaleTransitionAndAudit(t *testing.T) {
	t.Parallel()

	const studentID int64 = 703
	repo := &fakeStudentLifecycleRepo{
		pendingDue: []int64{studentID},
		currentStatuses: map[int64]string{
			studentID: studentStatusInactive,
		},
	}
	auditor := &fakeStudentLifecycleAuditor{}
	s := unitScheduler(&Scheduler{
		studentLifecycleRepo:  repo,
		studentLifecycleAudit: auditor})

	err := s.runActivateStudentsForTenantWithError(context.Background(), 7, time.Now())

	require.NoError(t, err)
	assert.Empty(t, repo.updates, "a concurrent staff status change must not be overwritten")
	assert.Empty(t, auditor.calls, "a skipped transition must not create a system audit entry")
}
