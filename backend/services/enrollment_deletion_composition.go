package services

import (
	"context"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

type enrollmentGuardianDirectory struct{ guardians peopledirectory.GuardianQuery }

func (d enrollmentGuardianDirectory) ListGuardiansByAccount(ctx context.Context, accountIDs []int64) ([]enrollmentCompose.DirectoryGuardian, error) {
	guardians, err := d.guardians.ListGuardiansByAccount(ctx, accountIDs)
	return toEnrollmentGuardians(guardians), err
}

func (d enrollmentGuardianDirectory) ListGuardiansByID(ctx context.Context, ids []int64) ([]enrollmentCompose.DirectoryGuardian, error) {
	guardians, err := d.guardians.ListGuardiansByID(ctx, ids)
	return toEnrollmentGuardians(guardians), err
}

func (d enrollmentGuardianDirectory) CountGuardianLinks(ctx context.Context, ids []int64) (map[int64]int, error) {
	return d.guardians.CountGuardianLinks(ctx, ids)
}

func toEnrollmentGuardians(guardians []peopledirectory.Guardian) []enrollmentCompose.DirectoryGuardian {
	result := make([]enrollmentCompose.DirectoryGuardian, 0, len(guardians))
	for _, guardian := range guardians {
		result = append(result, enrollmentCompose.DirectoryGuardian{ID: guardian.ID, AccountID: guardian.AccountID})
	}
	return result
}

// EnrollmentRejectedCleanup runs Enrollment's retention cleanup of rejected
// enrollments for the scheduler, which reads only the counts.
type EnrollmentRejectedCleanup struct {
	cleaner enrollmentOwner.RejectedEnrollmentCleaner
}

// CleanupRejectedEnrollments runs the cleanup for the tenant in context.
func (c EnrollmentRejectedCleanup) CleanupRejectedEnrollments(ctx context.Context) (int, int64, int64, error) {
	result, err := c.cleaner.CleanupRejectedEnrollments(ctx)
	return result.DeletedRequests, result.DeletedLateInvites, result.DeletedOutboxRows, err
}
