package parentaudience

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/parentannouncementaudience/contract"

	"github.com/uptrace/bun"
)

// The projection answers in its own vocabulary (ADR 0058); Communication's
// parent store maps the rows onto its repository contract.

// PendingApplicants supplies Enrollment's undecided applications: the
// pending_enrollment target reaches them although no student row exists yet.
type PendingApplicants interface {
	PendingAnnouncementApplicants(context.Context) ([]contract.PendingAnnouncementApplicant, error)
	PendingAnnouncementApplicantsForSchools(context.Context, []int64) ([]contract.PendingAnnouncementApplicant, error)
}

// GuardianLinks is the student-guardian relationship with its parents-portal
// permissions (#2756) as a row source the audience joins as sg in its own
// statements. The roots bind it to the guardian-link projection.
type GuardianLinks interface {
	GuardianLinkRows(db bun.IDB, tenantID int64) *bun.SelectQuery
	AllSchoolsGuardianLinkRows(db bun.IDB) *bun.SelectQuery
}
