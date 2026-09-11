package timetracking

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func (rs *StaffAdminResource) wakeOffboardingDocumentCleanup(ctx context.Context) {
	cleanupCtx := context.WithoutCancel(tenant.ContextWithoutTransaction(ctx))
	tenant.RegisterAfterCommit(ctx, func() {
		workerCtx, cancel := context.WithTimeout(cleanupCtx, time.Minute)
		defer cancel()
		if _, err := rs.runOffboardingDocumentCleanup(workerCtx); err != nil {
			rs.logger.Error("offboarding document cleanup incomplete", "error", err)
		}
	})
}

func (rs *StaffAdminResource) runOffboardingDocumentCleanup(ctx context.Context) (int, error) {
	claims, err := rs.OffboardingCleanup.Claim(ctx, 100)
	if err != nil {
		return 0, err
	}
	removed := 0
	var failures error
	for _, claim := range claims {
		count, cleanupErr := rs.cleanupClaimedStaffDocuments(ctx, claim.StaffID)
		removed += count
		finished, finishErr := rs.OffboardingCleanup.Finish(ctx, claim, cleanupErr == nil)
		if finishErr == nil && !finished {
			finishErr = errors.New("offboarding document cleanup lease expired")
		}
		failures = errors.Join(failures, cleanupErr, finishErr)
		if cleanupErr != nil || finishErr != nil {
			rs.logger.Warn("offboarding document cleanup retry pending",
				"staff_id", claim.StaffID,
				"attempt", claim.Attempts,
			)
		}
	}
	backlog, err := rs.OffboardingCleanup.Backlog(ctx)
	if err == nil {
		rs.logger.Debug("offboarding document cleanup backlog",
			"pending", backlog.Pending,
			"oldest_age_seconds", backlog.OldestAgeSeconds,
		)
	}
	return removed, errors.Join(failures, err)
}

func (rs *StaffAdminResource) cleanupClaimedStaffDocuments(ctx context.Context, staffID int64) (int, error) {
	var documents []workforce.StaffDocument
	var uploads []workforce.StaffDocumentFileCleanup
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		var err error
		documents, err = rs.StaffDocumentService.ListStaffDocumentsPendingFileCleanup(txCtx, staffID)
		if err != nil {
			return err
		}
		uploads, err = rs.StaffDocumentService.ListQueuedStaffDocumentFileCleanup(txCtx, staffID)
		return err
	})
	if err != nil {
		return 0, err
	}
	removed := 0
	var failures error
	for _, document := range documents {
		if err := rs.removeStoredDocumentForTenant(document.TenantID, document.FilenameStored); err != nil {
			failures = errors.Join(failures, err)
			continue
		}
		if err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
			return rs.StaffDocumentService.MarkStaffDocumentFileDeleted(txCtx, document.ID)
		}); err != nil {
			failures = errors.Join(failures, err)
			continue
		}
		removed++
	}
	for _, upload := range uploads {
		if err := rs.removeStoredDocumentForTenant(upload.TenantID, upload.FilenameStored); err != nil {
			failures = errors.Join(failures, err)
			continue
		}
		if err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
			return rs.StaffDocumentService.MarkQueuedStaffDocumentFileCleanupComplete(txCtx, upload.ID)
		}); err != nil {
			failures = errors.Join(failures, err)
			continue
		}
		removed++
	}
	if failures != nil {
		return removed, fmt.Errorf("offboarding staff document cleanup: %w", failures)
	}
	return removed, nil
}
