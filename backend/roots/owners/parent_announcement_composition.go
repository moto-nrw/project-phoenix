package owners

import (
	"context"

	recordports "github.com/moto-nrw/project-phoenix/modules/communication/ports"

	enrollmentCapability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// AnnouncementEnrollmentQueries is the Enrollment capability the parent
// announcement audience needs: the undecided applications the
// pending_enrollment target reaches.
type AnnouncementEnrollmentQueries interface {
	PendingAnnouncementApplicants(context.Context) ([]enrollmentCapability.PendingAnnouncementApplicant, error)
	PendingAnnouncementApplicantsForSchools(context.Context, []int64) ([]enrollmentCapability.PendingAnnouncementApplicant, error)
}

// ParentAnnouncementApplicants feeds Communication's announcement audience
// Enrollment's pending applicants as plain values; the caller composes the
// store over it with parentstore.NewParentAnnouncementRepository.
func ParentAnnouncementApplicants(enrollment AnnouncementEnrollmentQueries) recordports.PendingApplicantSource {
	if enrollment == nil {
		panic("parent announcements require Enrollment audience queries")
	}
	return pendingApplicantSource{enrollment: enrollment}
}

type pendingApplicantSource struct{ enrollment AnnouncementEnrollmentQueries }

func (s pendingApplicantSource) PendingAnnouncementApplicants(ctx context.Context) ([]recordports.PendingAnnouncementApplicant, error) {
	rows, err := s.enrollment.PendingAnnouncementApplicants(ctx)
	return pendingApplicants(rows), err
}

func (s pendingApplicantSource) PendingAnnouncementApplicantsForSchools(ctx context.Context, schoolIDs []int64) ([]recordports.PendingAnnouncementApplicant, error) {
	rows, err := s.enrollment.PendingAnnouncementApplicantsForSchools(ctx, schoolIDs)
	return pendingApplicants(rows), err
}

func pendingApplicants(rows []enrollmentCapability.PendingAnnouncementApplicant) []recordports.PendingAnnouncementApplicant {
	values := make([]recordports.PendingAnnouncementApplicant, 0, len(rows))
	for _, row := range rows {
		values = append(values, recordports.PendingAnnouncementApplicant{
			TenantID: row.TenantID, GuardianFirstName: row.GuardianFirstName, GuardianLastName: row.GuardianLastName,
			GuardianAccountID: row.GuardianAccountID, GuardianEmail: row.GuardianEmail,
		})
	}
	return values
}
