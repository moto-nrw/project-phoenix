package services

import (
	"context"
	"time"

	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	enrollmentCapability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/uptrace/bun"
)

// AnnouncementEnrollmentQueries is the Enrollment capability the parent
// announcement audience needs: the undecided applications the
// pending_enrollment target reaches.
type AnnouncementEnrollmentQueries interface {
	PendingAnnouncementApplicants(context.Context) ([]enrollmentCapability.PendingAnnouncementApplicant, error)
	PendingAnnouncementApplicantsForSchools(context.Context, []int64) ([]enrollmentCapability.PendingAnnouncementApplicant, error)
}

// NewParentAnnouncementRepository composes Communication's announcement store
// and audience projection behind the existing repository contract, feeding it
// Enrollment's pending applicants as plain values.
func NewParentAnnouncementRepository(db *bun.DB, enrollment AnnouncementEnrollmentQueries, clocks ...func() time.Time) userModels.ParentAnnouncementRepository {
	if enrollment == nil {
		panic("parent announcements require Enrollment audience queries")
	}
	return parentStore.NewParentAnnouncementRepository(db, pendingApplicantSource{enrollment: enrollment}, clocks...)
}

type pendingApplicantSource struct{ enrollment AnnouncementEnrollmentQueries }

func (s pendingApplicantSource) PendingAnnouncementApplicants(ctx context.Context) ([]parentStore.PendingAnnouncementApplicant, error) {
	rows, err := s.enrollment.PendingAnnouncementApplicants(ctx)
	return pendingApplicants(rows), err
}

func (s pendingApplicantSource) PendingAnnouncementApplicantsForSchools(ctx context.Context, schoolIDs []int64) ([]parentStore.PendingAnnouncementApplicant, error) {
	rows, err := s.enrollment.PendingAnnouncementApplicantsForSchools(ctx, schoolIDs)
	return pendingApplicants(rows), err
}

func pendingApplicants(rows []enrollmentCapability.PendingAnnouncementApplicant) []parentStore.PendingAnnouncementApplicant {
	values := make([]parentStore.PendingAnnouncementApplicant, 0, len(rows))
	for _, row := range rows {
		values = append(values, parentStore.PendingAnnouncementApplicant{
			TenantID: row.TenantID, GuardianFirstName: row.GuardianFirstName, GuardianLastName: row.GuardianLastName,
			GuardianAccountID: row.GuardianAccountID, GuardianEmail: row.GuardianEmail,
		})
	}
	return values
}
