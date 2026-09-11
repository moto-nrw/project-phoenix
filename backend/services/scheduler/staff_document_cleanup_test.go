package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

type staffDocumentFileCleanerStub struct {
	calls int
	err   error
}

func (s *staffDocumentFileCleanerStub) CleanupOrphanedStaffDocumentFiles(context.Context) (int, error) {
	s.calls++
	return 1, s.err
}

func TestStaffDocumentFileCleanupRunsWithoutUITraffic(t *testing.T) {
	t.Parallel()

	cleaner := &staffDocumentFileCleanerStub{}
	s := unitScheduler(&Scheduler{
		staffDocumentFileCleaner: cleaner,
		done:                     make(chan struct{}),
		logger:                   slog.Default()})

	task := &ScheduledTask{Name: "staff-document-file-cleanup"}

	s.checkAndRunStaffDocumentFileCleanup(context.Background(), task)

	assert.Equal(t, 1, cleaner.calls)
	assert.False(t, task.Running)
}

func TestStaffDocumentFileCleanupKeepsPartialProgress(t *testing.T) {
	t.Parallel()

	cleaner := &staffDocumentFileCleanerStub{err: errors.New("remove failed")}
	s := unitScheduler(&Scheduler{
		staffDocumentFileCleaner: cleaner,
		done:                     make(chan struct{}),
		logger:                   slog.Default()})

	// A failed file removal must not abort the tenant transaction: the marks
	// for the files that were removed have to commit.
	err := s.cleanupStaffDocumentFilesForTenant(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, 1, cleaner.calls)
}

func TestStaffDocumentFileCleanupAllowsRetryAfterFailure(t *testing.T) {
	t.Parallel()

	cleaner := &staffDocumentFileCleanerStub{err: errors.New("remove failed")}
	s := unitScheduler(&Scheduler{
		staffDocumentFileCleaner: cleaner,
		done:                     make(chan struct{}),
		logger:                   slog.Default()})

	task := &ScheduledTask{Name: "staff-document-file-cleanup"}

	s.checkAndRunStaffDocumentFileCleanup(context.Background(), task)
	s.checkAndRunStaffDocumentFileCleanup(context.Background(), task)

	assert.Equal(t, 2, cleaner.calls)
	assert.False(t, task.Running)
}
