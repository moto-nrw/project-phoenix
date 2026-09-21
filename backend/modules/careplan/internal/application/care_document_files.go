package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// The document file bookkeeping: an upload intent is recorded before its
// object is written, and the scheduler's sweep removes the bytes that no
// committed row still needs. File Storage moves the bytes; Care Plan decides
// which ones may go and settles its rows.

func (s *StudentDocuments) QueueStudentDocumentFileCleanup(ctx context.Context, studentID int64, storedName string) error {
	if studentID <= 0 || strings.TrimSpace(storedName) == "" {
		return fmt.Errorf("%w: cleanup file details are required", careplan.ErrStudentDocumentInvalid)
	}
	cleanup := careplan.CareDocumentCleanup{
		OwnerID: studentID, FilenameStored: storedName, RetryAfter: time.Now().Add(studentDocumentCleanupDelay),
	}
	return s.transaction(ctx, func(txCtx context.Context) error {
		_, err := s.records.QueueCareDocumentCleanup(txCtx, cleanup)
		return err
	})
}

// SweepStudentDocumentFiles runs one recovery pass over the tenant. It covers
// the two cases the request-scoped hooks cannot: a process that died between
// the object write and the metadata commit, and a child deleted while some of
// their documents still had bytes on disk. A row is settled only after its
// object is gone, so a failed removal is retried by the next pass.
func (s *StudentDocuments) SweepStudentDocumentFiles(ctx context.Context, remove careplan.StudentDocumentFileRemover) (careplan.StudentDocumentFileSweep, error) {
	var sweep careplan.StudentDocumentFileSweep
	if remove == nil {
		return sweep, errors.New("care plan documents: file remover is required")
	}
	var listErr error
	documents, err := s.records.ListDeletedCareDocuments(ctx, 0, nil)
	if err != nil {
		listErr = fmt.Errorf("list deleted student documents: %w", err)
	} else {
		sweep.DeletedListed = len(documents)
		for _, document := range documents {
			s.sweepDeletedDocument(ctx, remove, document, &sweep)
		}
	}
	cleanups, err := s.records.ListCareDocumentCleanups(ctx, nil)
	if err != nil {
		return sweep, errors.Join(listErr, fmt.Errorf("list queued student document cleanups: %w", err))
	}
	sweep.OrphansListed = len(cleanups)
	for _, cleanup := range cleanups {
		s.sweepOrphanedUpload(ctx, remove, cleanup, &sweep)
	}
	return sweep, listErr
}

func (s *StudentDocuments) sweepDeletedDocument(ctx context.Context, remove careplan.StudentDocumentFileRemover, document careplan.CareDocument, sweep *careplan.StudentDocumentFileSweep) {
	failure := careplan.StudentDocumentFileFailure{StudentID: document.StudentID, DocumentID: document.ID}
	if err := remove(ctx, document.TenantID, document.FilenameStored); err != nil {
		failure.Stage, failure.Err = careplan.StudentDocumentSweepRemove, err
		sweep.Failures = append(sweep.Failures, failure)
		return
	}
	if err := s.records.MarkCareDocumentFileDeleted(ctx, document.ID); err != nil {
		failure.Stage, failure.Err = careplan.StudentDocumentSweepSettle, err
		sweep.Failures = append(sweep.Failures, failure)
		return
	}
	sweep.Removed++
}

func (s *StudentDocuments) sweepOrphanedUpload(ctx context.Context, remove careplan.StudentDocumentFileRemover, cleanup careplan.CareDocumentCleanup, sweep *careplan.StudentDocumentFileSweep) {
	failure := careplan.StudentDocumentFileFailure{StudentID: cleanup.OwnerID, CleanupID: cleanup.ID}
	if err := remove(ctx, cleanup.TenantID, cleanup.FilenameStored); err != nil {
		failure.Stage, failure.Err = careplan.StudentDocumentSweepRemove, err
		sweep.Failures = append(sweep.Failures, failure)
		return
	}
	if err := s.records.CompleteCareDocumentCleanup(ctx, cleanup.ID); err != nil {
		failure.Stage, failure.Err = careplan.StudentDocumentSweepSettle, err
		sweep.Failures = append(sweep.Failures, failure)
		return
	}
	sweep.Removed++
}
