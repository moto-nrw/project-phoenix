package ports

import "context"

type OrganizationTargets interface {
	CountOrganizationsByID(context.Context, []int64) (int, error)
	CountSchoolsByID(context.Context, []int64) (int, error)
}
