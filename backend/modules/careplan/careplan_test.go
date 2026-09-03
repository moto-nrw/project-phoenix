package careplan

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingEngine struct {
	engine
	offeringFilter CareOfferingFilter
	create         CreateCareOffering
}

func (e *recordingEngine) ListCareOfferings(_ context.Context, filter CareOfferingFilter) ([]CareOffering, error) {
	e.offeringFilter = filter
	return nil, nil
}

func (e *recordingEngine) CreateCareOffering(_ context.Context, input CreateCareOffering) (CareOffering, error) {
	e.create = input
	return CareOffering{}, nil
}

func TestFacadeRejectsInvalidInputBeforePersistence(t *testing.T) {
	t.Parallel()
	module := NewModule(&recordingEngine{})

	_, err := module.FindCareOffering(context.Background(), 0)
	require.ErrorIs(t, err, ErrInvalidCareOffering)
	_, err = module.ListCareOfferings(context.Background(), CareOfferingFilter{Order: "random"})
	require.ErrorIs(t, err, ErrInvalidCareOffering)
	_, err = module.ListOfferingChanges(context.Background(), OfferingChangeFilter{Statuses: []string{"unknown"}})
	require.ErrorIs(t, err, ErrInvalidOfferingChange)
	_, err = module.CreateOfferingChange(context.Background(), OfferingChangeRequest{
		StudentID: 1, RequestChildID: 2, SubmittedBy: 3, EffectiveFrom: "2030-09-01",
		Status: OfferingChangePending, Payload: json.RawMessage(`[]`),
	})
	require.ErrorIs(t, err, ErrInvalidOfferingChange)
	require.ErrorIs(t, module.UpdateOfferingChangeEffectiveFrom(context.Background(), 1, "03.09.2026"), ErrInvalidOfferingChange)
	assert.Equal(t, "invalid", ErrorCode(err))
}

func TestFacadeNormalizesClosedFiltersAndTriggerIDs(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := NewModule(engine)

	_, err := module.ListCareOfferings(context.Background(), CareOfferingFilter{IDs: []int64{3, 0, 3, 2}})
	require.NoError(t, err)
	assert.Equal(t, []int64{3, 2}, engine.offeringFilter.IDs)
	assert.Equal(t, OfferingOrderCatalog, engine.offeringFilter.Order)

	_, err = module.CreateCareOffering(context.Background(), CreateCareOffering{CareOfferingFields: CareOfferingFields{
		PhaseID: 1, Name: " Ganztag ", AvailableDays: []string{"MON"}, AutoAddTriggerOfferingIDs: []int64{7, -1, 7, 9},
	}})
	require.NoError(t, err)
	assert.Equal(t, []int64{7, 9}, engine.create.AutoAddTriggerOfferingIDs)
	assert.Equal(t, "Ganztag", engine.create.Name)
	assert.Equal(t, []string{"mon"}, engine.create.AvailableDays)
	assert.Equal(t, "fixed", engine.create.DaysOfWeekMode)
	assert.Equal(t, "optional", engine.create.SelectionRule)
}

func TestFacadeRequiresAnEngine(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { NewModule(nil) })
}
