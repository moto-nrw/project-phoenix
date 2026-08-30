package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type statusAuditCall struct {
	studentID int64
	before    userModels.StudentStatus
	after     userModels.StudentStatus
}

type fakeStudentLifecycleAuditor struct {
	calls []statusAuditCall
	err   error
}

func (f *fakeStudentLifecycleAuditor) RecordSystemStatusChange(
	_ context.Context,
	studentID int64,
	before userModels.StudentStatus,
	after userModels.StudentStatus,
) error {
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, statusAuditCall{studentID: studentID, before: before, after: after})
	return nil
}

func TestRunActivateStudentsForTenant_AuditsSystemTransition(t *testing.T) {
	t.Parallel()

	student := &userModels.Student{Status: userModels.StudentStatusPending}
	student.ID = 701
	repo := &fakeStudentLifecycleRepo{pendingDue: []*userModels.Student{student}}
	auditor := &fakeStudentLifecycleAuditor{}
	s := unitScheduler(&Scheduler{
		studentLifecycleRepo:  repo,
		studentLifecycleAudit: auditor})

	err := s.runActivateStudentsForTenantWithError(context.Background(), 7, time.Now())

	require.NoError(t, err)
	require.Len(t, auditor.calls, 1)
	assert.Equal(t, int64(701), auditor.calls[0].studentID)
	assert.Equal(t, userModels.StudentStatusPending, auditor.calls[0].before)
	assert.Equal(t, userModels.StudentStatusActive, auditor.calls[0].after)
}

func TestRunActivateStudentsForTenant_PropagatesAuditFailure(t *testing.T) {
	t.Parallel()

	student := &userModels.Student{Status: userModels.StudentStatusActive}
	student.ID = 702
	repo := &fakeStudentLifecycleRepo{activeDue: []*userModels.Student{student}}
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

	student := &userModels.Student{Status: userModels.StudentStatusPending}
	student.ID = 703
	repo := &fakeStudentLifecycleRepo{
		pendingDue: []*userModels.Student{student},
		currentStatuses: map[int64]userModels.StudentStatus{
			student.ID: userModels.StudentStatusInactive,
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
