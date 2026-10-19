package compose

import (
	"errors"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
)

type MasterDataDirectory = careplan.MasterDataDirectory
type MasterDataFieldChange = careplan.MasterDataFieldChange
type MasterDataFieldFacts = careplan.MasterDataFieldFacts

type MasterDataReviewDependencies struct {
	Requests careplan.StudentDataRequestQuery
	People   MasterDataDirectory
	Scope    ReviewScopeResolver
	Today    func() careplan.Date
}

// NewStudentDataRequestQueries composes the read-only queue source without
// requiring mutation locks or unrelated status-day collaborators.
func NewStudentDataRequestQueries(db *bun.DB, directory StudentDirectory, observe func(Observation)) (careplan.StudentDataRequestQuery, error) {
	if db == nil || directory == nil || observe == nil {
		return nil, errors.New("care plan student data request queries: database, student directory and observer are required")
	}
	return application.NewRequestQueueQueries(postgres.NewRequestStore(postgres.New(carePlanDatabase(db)), directory), observe), nil
}

// NewCareScheduleRequestQueries binds only the storage reads of the next
// review queue, without constructing any decision or schedule mutation path.
func NewCareScheduleRequestQueries(db *bun.DB, directory StudentDirectory, observe func(Observation)) (careplan.CareScheduleRequestQuery, error) {
	if db == nil || directory == nil || observe == nil {
		return nil, errors.New("care plan schedule request queries: database, student directory and observer are required")
	}
	return application.NewRequestQueueQueries(postgres.NewRequestStore(postgres.New(carePlanDatabase(db)), directory), observe), nil
}

func NewMasterDataReviews(deps MasterDataReviewDependencies) (careplan.MasterDataReviewQuery, error) {
	if deps.Requests == nil || deps.People == nil || deps.Scope == nil || deps.Today == nil {
		return nil, errors.New("care plan master data reviews: requests, people, scope, and clock are required")
	}
	return application.NewMasterDataReviews(deps.Requests, deps.People, deps.Scope, deps.Today), nil
}
