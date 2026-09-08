// Package classdaytest provides the legacy schedule adapters the class-day
// projection's integration tests still compose while the care-day derivation
// and the effective-time services remain in the retained schedule package.
package classdaytest

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

// NewApprovedOfferingProjection composes the enrollment-owned approved
// offering projection the pickup baselines read.
func NewApprovedOfferingProjection(db *bun.DB) (*services.ApprovedOfferingTestProjection, error) {
	return services.NewOwnerApprovedOfferingTestProjection(db)
}
