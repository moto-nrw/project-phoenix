// Package slotlisttest provides the legacy schedule adapters needed by
// slot-list integration tests while the class-day view is being migrated.
package slotlisttest

import (
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/uptrace/bun"
)

// NewStudentScheduleRepositories composes the production compatibility
// adapters without exposing the complete legacy repository factory to tests.
func NewStudentScheduleRepositories(db *bun.DB) repositories.StudentScheduleRepositories {
	return repositories.NewStudentScheduleRepositories(db)
}

func NewApprovedOfferingProjection(db *bun.DB) (*services.ApprovedOfferingTestProjection, error) {
	return services.NewOwnerApprovedOfferingTestProjection(db)
}
