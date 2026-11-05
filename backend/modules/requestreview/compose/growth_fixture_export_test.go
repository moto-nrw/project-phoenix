package compose

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
	reviewbindings "github.com/moto-nrw/project-phoenix/roots/requestreviewbinding"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// NewGrowthQuery reuses the existing real queue fixtures for external DB tests.
// This test-only composition adds no production capability or runtime branch.
func NewGrowthQuery(t *testing.T, db *bun.DB, svc ReviewComposition, master careplan.MasterDataReviewQuery, schedule careplan.CareScheduleReviewQuery, offering careplan.OfferingReviewQuery) (requestreview.Query, error) {
	t.Helper()
	require.Same(t, testpkg.SetupTestDB(t), db)
	students, err := composeStudentDirectory(t, db, svc.PeopleDirectory)
	if err != nil {
		return nil, err
	}
	corrections, err := composeCorrectionLog(t, db, svc.PeopleDirectory, func(context.Context) bool { return true })
	if err != nil {
		return nil, err
	}
	return requestreview.NewChecked(requestreview.Dependencies{
		Queues: requestreview.Queues{DirectCorrections: corrections, MasterData: nativeMasterQueue(master), CareSchedule: nativeCareQueue(schedule, reviewToday), Offering: nativeOfferingQueue(offering, reviewToday), Excused: nativeExcusedQueue(svc.ExcusedRequests, reviewToday)},
		Access: queueTestAccess{}, Students: students, FamilyProtection: svc.FamilyProtection,
		Submitters: reviewbindings.NewSubmitterDirectory(svc.PeopleDirectory),
	})
}

// GrowthWriteContext grants the existing fixture's queue access to its tenant.
func GrowthWriteContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, queueWriteKey{}, true)
}

// GrowthQuery and its request/response aliases keep public owner types inside
// the policy-approved internal fixture binding, not the external test imports.
type GrowthQuery = requestreview.Query
type GrowthListQuery = requestreview.ListQuery
type GrowthPage = requestreview.Page

const GrowthMaxLimit = requestreview.MaxLimit

func GrowthRequestTypes() []string {
	return []string{requestreview.TypeMasterData, requestreview.TypeCareSchedule, requestreview.TypeOffering, requestreview.TypeExcused}
}

// WithGrowthQuery binds the existing typed root proof before handing its query
// to the external DB contract test.
func WithGrowthQuery(t *testing.T, check func(*bun.DB, GrowthQuery)) {
	t.Helper()
	ComposeReviewSources(t, ReviewProof(func(db *bun.DB, svc ReviewComposition, master careplan.MasterDataReviewQuery, schedule careplan.CareScheduleReviewQuery, offering careplan.OfferingReviewQuery) {
		query, err := NewGrowthQuery(t, db, svc, master, schedule, offering)
		require.NoError(t, err)
		check(db, query)
	}))
}
