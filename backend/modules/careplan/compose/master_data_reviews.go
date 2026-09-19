package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/uptrace/bun"
)

type MasterDataDirectory = ports.MasterDataDirectory
type MasterDataFieldChange = ports.MasterDataFieldChange
type MasterDataFieldFacts = ports.MasterDataFieldFacts

type MasterDataReviewDependencies struct {
	Requests careplan.StudentDataRequestQuery
	People   MasterDataDirectory
	Scope    ReviewScopeResolver
	Today    func() careplan.Date
}

// NewStudentDataRequestQueries composes the read-only queue source without
// requiring mutation locks or unrelated status-day collaborators.
func NewStudentDataRequestQueries(db *bun.DB, observe func(Observation)) (careplan.StudentDataRequestQuery, error) {
	if db == nil || observe == nil {
		return nil, errors.New("care plan student data request queries: database and observer are required")
	}
	return requestQueueQueries{store: postgres.NewRequestStore(postgres.New(carePlanDatabase(db))), observe: observe}, nil
}

type requestQueueQueries struct {
	store interface {
		ListStudentDataRequests(context.Context, careplan.StudentDataRequestFilter) ([]careplan.StudentDataChangeRequest, RequestStoreStats, error)
		ListCareScheduleRequests(context.Context, careplan.CareScheduleRequestFilter) ([]careplan.CareScheduleChangeRequest, RequestStoreStats, error)
	}
	observe func(Observation)
}

func (q requestQueueQueries) ListStudentDataRequests(ctx context.Context, filter careplan.StudentDataRequestFilter) ([]careplan.StudentDataChangeRequest, error) {
	started := time.Now()
	rows, stats, err := q.store.ListStudentDataRequests(ctx, filter)
	q.observe(Observation{Operation: "list_student_data_requests", Duration: time.Since(started), Stats: stats, Err: err})
	return rows, err
}

// NewCareScheduleRequestQueries binds only the storage reads of the next
// review queue, without constructing any decision or schedule mutation path.
func NewCareScheduleRequestQueries(db *bun.DB, observe func(Observation)) (careplan.CareScheduleRequestQuery, error) {
	if db == nil || observe == nil {
		return nil, errors.New("care plan schedule request queries: database and observer are required")
	}
	return requestQueueQueries{store: postgres.NewRequestStore(postgres.New(carePlanDatabase(db))), observe: observe}, nil
}

func (q requestQueueQueries) ListCareScheduleRequests(ctx context.Context, filter careplan.CareScheduleRequestFilter) ([]careplan.CareScheduleChangeRequest, error) {
	started := time.Now()
	rows, stats, err := q.store.ListCareScheduleRequests(ctx, filter)
	q.observe(Observation{Operation: "list_care_schedule_requests", Duration: time.Since(started), Stats: stats, Err: err})
	return rows, err
}

func NewMasterDataReviews(deps MasterDataReviewDependencies) (careplan.MasterDataReviewQuery, error) {
	if deps.Requests == nil || deps.People == nil || deps.Scope == nil || deps.Today == nil {
		return nil, errors.New("care plan master data reviews: requests, people, scope, and clock are required")
	}
	return application.NewMasterDataReviews(deps.Requests, deps.People, deps.Scope, deps.Today), nil
}
