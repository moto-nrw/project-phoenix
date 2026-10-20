package application

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

// unreachableStore panics on every call: the inputs below are rejected
// before persistence.
type unreachableStore struct{ ExceptionStore }

func newUnreachableService() *Service {
	return NewService(Dependencies{Store: unreachableStore{}, Observe: func(ports.Observation) {}})
}

func TestFacadeRejectsInvalidInputBeforePersistence(t *testing.T) {
	t.Parallel()
	module := newUnreachableService()

	_, err := module.FindCareOffering(context.Background(), 0)
	require.ErrorIs(t, err, careplan.ErrInvalidCareOffering)
	_, err = module.ListCareOfferings(context.Background(), careplan.CareOfferingFilter{Order: "random"})
	require.ErrorIs(t, err, careplan.ErrInvalidCareOffering)
	_, err = module.ListOfferingChanges(context.Background(), careplan.OfferingChangeFilter{Statuses: []string{"unknown"}})
	require.ErrorIs(t, err, careplan.ErrInvalidOfferingChange)
	_, err = module.CreateOfferingChange(context.Background(), careplan.OfferingChangeRequest{
		StudentID: 1, RequestChildID: 2, SubmittedBy: 3, EffectiveFrom: "2030-09-01",
		Status: careplan.OfferingChangePending, Payload: json.RawMessage(`[]`),
	})
	require.ErrorIs(t, err, careplan.ErrInvalidOfferingChange)
	require.ErrorIs(t, module.UpdateOfferingChangeEffectiveFrom(context.Background(), 1, "03.09.2026"), careplan.ErrInvalidOfferingChange)
	require.ErrorIs(t, module.ReplaceCompanionEdges(context.Background(), 1, []careplan.CompanionEdge{{StudentLowID: 1, StudentHighID: 1, Weekday: 1}}), careplan.ErrInvalidCompanion)
	_, err = module.CreateCareDocument(context.Background(), careplan.CareDocument{StudentID: 1})
	require.ErrorIs(t, err, careplan.ErrInvalidCareDocument)
	err = module.RecordCareExitSourceRemovals(context.Background(), []careplan.CareExitSourceRemoval{{StudentID: 1, SourceRowID: 2, Snapshot: json.RawMessage(`bad`)}})
	require.ErrorIs(t, err, careplan.ErrInvalidCareExit)
	assert.Equal(t, "invalid", careplan.ErrorCode(err))
	require.ErrorIs(t, module.RecordCareExitSourceRemovals(context.Background(), []careplan.CareExitSourceRemoval{{StudentID: 1, SourceRowID: 2, Kind: "unknown", Snapshot: json.RawMessage(`{}`)}}), careplan.ErrInvalidCareExit)
	require.ErrorIs(t, module.RecordCareExitRemovals(context.Background(), []careplan.CareExitRemoval{{StudentID: 1, Kind: careplan.CareExitRemovalRoster}}), careplan.ErrInvalidCareExit)
}

// The service hands these normalized values to the store; the composition
// test TestCareOfferingNormalizationReachesTheStore covers that hand-over.
func TestFacadeNormalizesClosedFiltersAndTriggerIDs(t *testing.T) {
	t.Parallel()

	filter, err := normalizeCareOfferingFilter(careplan.CareOfferingFilter{IDs: []int64{3, 0, 3, 2}})
	require.NoError(t, err)
	assert.Equal(t, []int64{3, 2}, filter.IDs)
	assert.Equal(t, careplan.OfferingOrderCatalog, filter.Order)

	create, err := normalizeCareOfferingFields(careplan.CareOfferingFields{
		PhaseID: 1, Name: " Ganztag ", AvailableDays: []string{"MON"}, AutoAddTriggerOfferingIDs: []int64{7, -1, 7, 9},
	})
	require.NoError(t, err)
	create.AutoAddTriggerOfferingIDs = normalizedTriggers(0, create.AutoAddTriggerOfferingIDs)
	assert.Equal(t, []int64{7, 9}, create.AutoAddTriggerOfferingIDs)
	assert.Equal(t, "Ganztag", create.Name)
	assert.Equal(t, []string{"mon"}, create.AvailableDays)
	assert.Equal(t, "fixed", create.DaysOfWeekMode)
	assert.Equal(t, "optional", create.SelectionRule)
}

func TestServiceRequiresAStoreAndAnObserver(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { New(nil, nil) })
}

func TestStatusDayRetentionRefusesAWindowThatIsNotPositive(t *testing.T) {
	t.Parallel()
	module := newUnreachableService()
	for _, days := range []int{0, -3} {
		_, err := module.DeleteExpiredStudentStatusDays(context.Background(), days)
		require.ErrorIs(t, err, careplan.ErrStatusDayRetentionWindow)
	}
}
