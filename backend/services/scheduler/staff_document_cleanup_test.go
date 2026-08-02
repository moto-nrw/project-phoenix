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
	cleaner := &staffDocumentFileCleanerStub{}
	s := &Scheduler{
		staffDocumentFileCleaner: cleaner,
		done:                     make(chan struct{}),
		logger:                   slog.Default(),
	}
	task := &ScheduledTask{Name: "staff-document-file-cleanup"}

	s.checkAndRunStaffDocumentFileCleanup(task)

	assert.Equal(t, 1, cleaner.calls)
	assert.False(t, task.Running)
}

func TestStaffDocumentFileCleanupKeepsPartialProgress(t *testing.T) {
	cleaner := &staffDocumentFileCleanerStub{err: errors.New("remove failed")}
	s := &Scheduler{
		staffDocumentFileCleaner: cleaner,
		done:                     make(chan struct{}),
		logger:                   slog.Default(),
	}

	// A failed file removal must not abort the tenant transaction: the marks
	// for the files that were removed have to commit.
	err := s.cleanupStaffDocumentFilesForTenant(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, 1, cleaner.calls)
}

func TestStaffDocumentFileCleanupAllowsRetryAfterFailure(t *testing.T) {
	cleaner := &staffDocumentFileCleanerStub{err: errors.New("remove failed")}
	s := &Scheduler{
		staffDocumentFileCleaner: cleaner,
		done:                     make(chan struct{}),
		logger:                   slog.Default(),
	}
	task := &ScheduledTask{Name: "staff-document-file-cleanup"}

	s.checkAndRunStaffDocumentFileCleanup(task)
	s.checkAndRunStaffDocumentFileCleanup(task)

	assert.Equal(t, 2, cleaner.calls)
	assert.False(t, task.Running)
}
