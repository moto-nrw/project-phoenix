package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/inbound/students"
	schoolMembershipModule "github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// childQuotaStudentsReader adapts the School Membership Kinderkontingent read
// to the port api/students consumes for the Datenverwaltung (#3569).
type childQuotaStudentsReader struct {
	usages schoolMembershipModule.ChildQuotaUsages
}

func (r childQuotaStudentsReader) ChildQuotaUsage(ctx context.Context) (students.ChildQuotaUsage, bool, error) {
	usage, limited, err := r.usages.ChildQuotaUsage(ctx)
	return students.ChildQuotaUsage{Booked: usage.Booked, Occupied: usage.Occupied}, limited, err
}
