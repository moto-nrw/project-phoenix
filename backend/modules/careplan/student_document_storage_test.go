package careplan

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sweepingDocuments serves only the recovery pass of the child documents.
type sweepingDocuments struct {
	StudentDocuments
	sweep StudentDocumentFileSweep
	err   error
}

func (d *sweepingDocuments) SweepStudentDocumentFiles(ctx context.Context, remove StudentDocumentFileRemover) (StudentDocumentFileSweep, error) {
	if err := remove(ctx, 3, "stored.pdf"); err != nil {
		return StudentDocumentFileSweep{}, err
	}
	return d.sweep, d.err
}

func TestNewStudentDocumentFileCleanupRequiresEveryDependency(t *testing.T) {
	t.Parallel()
	removal := func(string, StudentDocuments, *slog.Logger) (StudentDocumentFileRemover, error) { return nil, nil }
	for name, build := range map[string]func() (*StudentDocumentFileCleanup, error){
		"documents": func() (*StudentDocumentFileCleanup, error) {
			return NewStudentDocumentFileCleanup(nil, removal, slog.Default())
		},
		"removal": func() (*StudentDocumentFileCleanup, error) {
			return NewStudentDocumentFileCleanup(&sweepingDocuments{}, nil, slog.Default())
		},
		"logger": func() (*StudentDocumentFileCleanup, error) {
			return NewStudentDocumentFileCleanup(&sweepingDocuments{}, removal, nil)
		},
	} {
		cleanup, err := build()
		assert.Nil(t, cleanup, name)
		assert.EqualError(t, err, "care plan student document file cleanup: all dependencies are required", name)
	}
}

func TestStudentDocumentFileCleanupReportsTheStorage(t *testing.T) {
	t.Parallel()
	removed, err := CleanupOrphanedStudentDocumentFiles(context.Background(), &sweepingDocuments{}, nil, slog.Default())
	assert.Zero(t, removed)
	assert.EqualError(t, err, "student document storage unavailable: student document storage is not configured")

	backendDown := errors.New("backend down")
	removed, err = CleanupOrphanedStudentDocumentFiles(context.Background(), &sweepingDocuments{},
		func(string, StudentDocuments, *slog.Logger) (StudentDocumentFileRemover, error) {
			return nil, backendDown
		}, slog.Default())
	assert.Zero(t, removed)
	require.ErrorIs(t, err, backendDown)
	assert.EqualError(t, err, "student document storage unavailable: backend down")
}

func TestStudentDocumentFileCleanupLogsWhatThePassLeft(t *testing.T) {
	t.Parallel()
	removeFailed, settleFailed, listFailed := errors.New("remove failed"), errors.New("settle failed"), errors.New("list failed")
	documents := &sweepingDocuments{err: listFailed, sweep: StudentDocumentFileSweep{
		Removed:       4,
		DeletedListed: StudentDocumentSweepBatchSize,
		OrphansListed: 1,
		Failures: []StudentDocumentFileFailure{
			{StudentID: 5, DocumentID: 6, Stage: StudentDocumentSweepRemove, Err: removeFailed},
			{StudentID: 5, CleanupID: 8, Stage: StudentDocumentSweepSettle, Err: settleFailed},
		},
	}}
	var kinds, removedNames []string
	removal := func(kind string, store StudentDocuments, _ *slog.Logger) (StudentDocumentFileRemover, error) {
		kinds = append(kinds, kind)
		assert.Same(t, documents, store)
		return func(_ context.Context, _ int64, storedName string) error {
			removedNames = append(removedNames, storedName)
			return nil
		}, nil
	}
	var logs bytes.Buffer
	cleanup, err := NewStudentDocumentFileCleanup(documents, removal, slog.New(slog.NewTextHandler(&logs, nil)))
	require.NoError(t, err)

	removed, err := cleanup.CleanupOrphanedStudentDocumentFiles(context.Background())

	assert.Equal(t, 4, removed)
	require.ErrorIs(t, err, listFailed)
	require.ErrorIs(t, err, removeFailed)
	require.ErrorIs(t, err, settleFailed)
	assert.Equal(t, []string{StudentDocumentStorageKind}, kinds)
	assert.Equal(t, []string{"stored.pdf"}, removedNames)
	output := logs.String()
	assert.Contains(t, output, `level=WARN msg="student document cleanup failed" student_id=5 document_id=6`)
	assert.Contains(t, output, `level=ERROR msg="student document orphan cleanup status update failed" student_id=5 cleanup_id=8`)
	assert.Contains(t, output, `msg="student document cleanup batch full, more pending" batch_size=200 source=deleted`)
	assert.NotContains(t, output, "source=orphan")
}
