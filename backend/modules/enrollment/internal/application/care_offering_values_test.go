package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// A route that saves an offering sets counts_as_care itself, so an explicit
// false must reach Care Plan as false instead of the column default (#2734).
func TestCareOfferingRowOfKeepsExplicitCountsAsCareFalse(t *testing.T) {
	t.Parallel()

	row, err := CareOfferingRowOf(&enrollment.CareOffering{PhaseID: 5678, Name: "Randstunde", CountsAsCare: false})
	require.NoError(t, err)
	assert.False(t, row.CountsAsCare)
	assert.True(t, row.CountsAsCareSet)

	offering, err := careOfferingFromRow(row)
	require.NoError(t, err)
	assert.False(t, offering.CountsAsCare)
}

// The availability rule travels as the owner stores it: decoded into the row
// and encoded back unchanged, and absent when the value carries none.
func TestCareOfferingRowOfRoundTripsTheAvailabilityRule(t *testing.T) {
	t.Parallel()

	rule := json.RawMessage(`{"match":"all","conditions":[{"source":"grade_level","operator":"in","value":[1,2]}]}`)
	row, err := CareOfferingRowOf(&enrollment.CareOffering{PhaseID: 5678, Name: "OGS", AvailabilityRule: rule})
	require.NoError(t, err)
	require.NotNil(t, row.AvailabilityRule)
	assert.JSONEq(t, string(rule), string(PublicCareOffering(row).AvailabilityRule))

	row, err = CareOfferingRowOf(&enrollment.CareOffering{PhaseID: 5678, Name: "OGS"})
	require.NoError(t, err)
	assert.Nil(t, row.AvailabilityRule)

	_, err = CareOfferingRowOf(&enrollment.CareOffering{PhaseID: 5678, Name: "OGS", AvailabilityRule: json.RawMessage(`{`)})
	require.Error(t, err)
}

type parentOfferingQuerySource struct {
	list func(context.Context, careplan.CareOfferingFilter) ([]careplan.CareOffering, error)
	find func(context.Context, int64, bool) (careplan.OfferingChangeRequest, error)
}

func (s parentOfferingQuerySource) ListCareOfferings(ctx context.Context, filter careplan.CareOfferingFilter) ([]careplan.CareOffering, error) {
	return s.list(ctx, filter)
}
func (s parentOfferingQuerySource) FindOfferingChange(ctx context.Context, id int64, lock bool) (careplan.OfferingChangeRequest, error) {
	return s.find(ctx, id, lock)
}

func TestParentCatalogQueriesKeepDecodedValuesAndLookupContract(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	price := 1200
	description := "care"
	value := careplan.CareOffering{ID: 41, Name: "OGS", Description: &description,
		DaysOfWeekMode: "fixed", AvailableDays: []string{"mon", "wed"}, PriceCents: &price,
		IncludesLunch: true, IncludesHolidayCare: true, SortOrder: 13}
	var sourceErr error
	calls := 0
	source := parentOfferingQuerySource{list: func(actual context.Context, filter careplan.CareOfferingFilter) ([]careplan.CareOffering, error) {
		calls++
		assert.Same(t, ctx, actual)
		assert.Equal(t, []int64{41}, filter.IDs)
		assert.Equal(t, careplan.OfferingOrderCatalog, filter.Order)
		assert.False(t, filter.LockForUpdate)
		return []careplan.CareOffering{value}, sourceErr
	}}
	var queries enrollment.ParentCareOfferingQueries = NewCareOfferingRecords(source)
	rows, err := queries.CareOfferingsByIDs(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, rows)
	assert.Empty(t, rows)
	assert.Zero(t, calls, "empty IDs must not read the complete catalog")
	rows, err = queries.CareOfferingsByIDs(ctx, []int64{41})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, value.Name, rows[0].Name)
	assert.Equal(t, value.Description, rows[0].Description)
	assert.Equal(t, value.AvailableDays, rows[0].AvailableDays)
	assert.Equal(t, value.PriceCents, rows[0].PriceCents)
	assert.True(t, rows[0].IncludesLunch)
	assert.True(t, rows[0].IncludesHolidayCare)
	assert.Equal(t, 13, rows[0].SortOrder)

	value.AvailabilityRule = json.RawMessage(`false`)
	rows, err = queries.CareOfferingsByIDs(ctx, []int64{41})
	assert.Nil(t, rows)
	require.ErrorContains(t, err, "failed to list care offerings by ids: decode care offering availability rule")
	sourceErr = errors.New("catalog unavailable")
	rows, err = queries.CareOfferingsByIDs(ctx, []int64{41})
	assert.Nil(t, rows)
	assert.ErrorIs(t, err, sourceErr)
}

func TestParentOfferingIdentityRetainsDecodingAndMissingRowErrors(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	value := careplan.OfferingChangeRequest{ID: 61, StudentID: 41, SubmittedBy: 23}
	var sourceErr error
	source := parentOfferingQuerySource{find: func(actual context.Context, id int64, lock bool) (careplan.OfferingChangeRequest, error) {
		assert.Same(t, ctx, actual)
		assert.Equal(t, int64(61), id)
		assert.False(t, lock)
		return value, sourceErr
	}}
	missingRow := errors.New("store missing row")
	var queries enrollment.ParentOfferingChangeQueries = NewOfferingChangeRecords(source, missingRow)
	identity, err := queries.OfferingChangeIdentity(ctx, 61)
	require.NoError(t, err)
	require.NotNil(t, identity)
	assert.Equal(t, int64(41), identity.StudentID)
	assert.Equal(t, int64(23), identity.SubmittedBy)
	value.Payload = json.RawMessage(`false`)
	identity, err = queries.OfferingChangeIdentity(ctx, 61)
	assert.Nil(t, identity)
	require.ErrorContains(t, err, "database error during find by id: decode offering change payload")
	value.Payload = nil
	value.DecisionSnapshot = json.RawMessage(`false`)
	identity, err = queries.OfferingChangeIdentity(ctx, 61)
	assert.Nil(t, identity)
	require.ErrorContains(t, err, "decode offering change decision snapshot")
	sourceErr = careplan.ErrOfferingChangeNotFound
	identity, err = queries.OfferingChangeIdentity(ctx, 61)
	assert.Nil(t, identity)
	assert.ErrorIs(t, err, missingRow)
	var marker interface{ RepositoryNotFound() }
	assert.ErrorAs(t, err, &marker)
	sourceErr = errors.New("lookup unavailable")
	_, err = queries.OfferingChangeIdentity(ctx, 61)
	assert.ErrorIs(t, err, sourceErr)
}
