//go:build testhooks

package api

import (
	"github.com/moto-ogs/project-phoenix/roots/owners"
	parentportal "github.com/moto-ogs/project-phoenix/workflows/parentportal"
	portalCompose "github.com/moto-ogs/project-phoenix/workflows/parentportal/compose"
)

// WithParentEnrollmentBudgetReads binds the same typed Enrollment reads as the
// production parent root, without exposing workflow-private port types to tests.
func WithParentEnrollmentBudgetReads(deps portalCompose.Dependencies, rows owners.EnrollmentRows) portalCompose.Dependencies {
	deps.CarePeriods = parentportal.NewCarePeriodReads(rows.ParentCarePeriods.CarePeriods)
	deps.OfferingHistory = parentportal.NewOfferingHistoryReads(rows.ParentOfferingHistory.OfferingHistory)
	return deps
}
