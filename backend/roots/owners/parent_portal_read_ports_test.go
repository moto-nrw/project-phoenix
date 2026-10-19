package owners

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type parentEnrollmentQuerySource struct {
	rows      []*careplan.ParentPortalEnrollment
	err       error
	observe   func(context.Context)
	accountID int64
}

func (s *parentEnrollmentQuerySource) ListByAccount(ctx context.Context, accountID int64) ([]*careplan.ParentPortalEnrollment, error) {
	if s.observe != nil {
		s.observe(ctx)
	}
	s.accountID = accountID
	return s.rows, s.err
}

func TestParentEnrollmentBindingPreservesRedactionFactsAndWireFields(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 8, 24, 10, 30, 0, 0, time.UTC)
	reason := "visible reason"
	source := &parentEnrollmentQuerySource{rows: []*careplan.ParentPortalEnrollment{nil, {
		RequestID: 11, TenantID: 23, StatusToken: "token", SubmittedAt: at, WithdrawnAt: &at,
		PhaseID: 31, PhaseName: "phase", ServiceStartDate: calendar.NewDate(2026, 8, 1), ServiceEndDate: calendar.NewDate(2027, 7, 31),
		ShowStatusReasonToParent: true, SchoolName: "School", SchoolSlug: "school",
		Children: []careplan.ParentPortalEnrollmentChild{{ChildID: 43, FirstName: "Alex", LastName: "Test", Status: "approved", StatusReason: &reason}},
	}}}
	reader := parentEnrollmentReads{source: source}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source.observe = func(actual context.Context) { assert.Same(t, ctx, actual) }
	rows, err := reader.ListByAccount(ctx, 53)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, int64(53), source.accountID)
	assert.Nil(t, rows[0])
	assert.True(t, rows[1].ShowStatusReasonToParent, "the workflow still needs the owner's redaction gate")
	before, err := json.Marshal(source.rows)
	require.NoError(t, err)
	after, err := json.Marshal(rows)
	require.NoError(t, err)
	assert.JSONEq(t, string(before), string(after))

	source.rows = nil
	source.err = errors.New("read failed")
	rows, err = reader.ListByAccount(ctx, 53)
	assert.Nil(t, rows)
	assert.ErrorIs(t, err, source.err)
	source.err = nil
	source.rows = []*careplan.ParentPortalEnrollment{}
	rows, err = reader.ListByAccount(ctx, 53)
	require.NoError(t, err)
	require.NotNil(t, rows)
	assert.Empty(t, rows)
}

func TestParentReadBindingsKeepOptionalSourcesNil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, NewParentChildReads(nil))
	assert.Nil(t, NewParentPhaseReads(nil))
	assert.Nil(t, NewParentEnrollmentReads(nil))
	assert.Nil(t, NewParentCareOfferingReads(nil))
	assert.Nil(t, NewParentOfferingChangeReads(nil))
}

type parentCareOfferingQuerySource struct {
	rows    []*enrollment.CareOffering
	err     error
	observe func(context.Context, []int64)
}

func (s *parentCareOfferingQuerySource) CareOfferingsByIDs(ctx context.Context, ids []int64) ([]*enrollment.CareOffering, error) {
	if s.observe != nil {
		s.observe(ctx, ids)
	}
	return s.rows, s.err
}

type parentOfferingChangeQuerySource struct {
	row     *enrollment.OfferingChangeIdentity
	err     error
	observe func(context.Context, int64)
}

func (s *parentOfferingChangeQuerySource) OfferingChangeIdentity(ctx context.Context, id int64) (*enrollment.OfferingChangeIdentity, error) {
	if s.observe != nil {
		s.observe(ctx, id)
	}
	return s.row, s.err
}

func TestParentOfferingBindingPreservesCatalogFactsAndReadResults(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	price := 1200
	description := "care"
	source := &parentCareOfferingQuerySource{rows: []*enrollment.CareOffering{nil, {
		ID: 41, Name: "OGS", Description: &description, DaysOfWeekMode: "fixed",
		AvailableDays: []string{"mon", "wed"}, PriceCents: &price, SortOrder: 13,
		IncludesLunch: true, IncludesHolidayCare: true,
	}, {DaysOfWeekMode: "parent_choice"}, {}}}
	source.observe = func(actual context.Context, ids []int64) {
		assert.Same(t, ctx, actual)
		assert.Equal(t, []int64{41}, ids)
	}
	reader := NewParentCareOfferingReads(source)
	rows, err := reader.ListByIDs(ctx, []int64{41})
	require.NoError(t, err)
	require.Len(t, rows, 4)
	assert.Nil(t, rows[0])
	assert.Equal(t, int64(41), rows[1].ID)
	assert.Equal(t, "OGS", rows[1].Name)
	assert.Equal(t, &description, rows[1].Description)
	assert.Equal(t, &price, rows[1].PriceCents)
	assert.Equal(t, 13, rows[1].SortOrder)
	assert.Equal(t, []string{"mon", "wed"}, rows[1].AvailableDays)
	assert.True(t, rows[1].IncludesLunch)
	assert.True(t, rows[1].IncludesHolidayCare)
	assert.True(t, rows[1].FixedDays)
	assert.False(t, rows[2].FixedDays)
	assert.False(t, rows[3].FixedDays)
	source.rows = nil
	source.err = errors.New("catalog failed")
	rows, err = reader.ListByIDs(ctx, []int64{41})
	assert.Nil(t, rows)
	assert.ErrorIs(t, err, source.err)
	source.rows = []*enrollment.CareOffering{}
	source.err = nil
	rows, err = reader.ListByIDs(ctx, []int64{41})
	require.NoError(t, err)
	require.NotNil(t, rows)
	assert.Empty(t, rows)
}

func TestParentOfferingIdentityBindingKeepsChildAndActorSeparate(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source := &parentOfferingChangeQuerySource{row: &enrollment.OfferingChangeIdentity{StudentID: 41, SubmittedBy: 23}}
	source.observe = func(actual context.Context, id int64) {
		assert.Same(t, ctx, actual)
		assert.Equal(t, int64(61), id)
	}
	reader := NewParentOfferingChangeReads(source)
	identity, err := reader.FindByID(ctx, 61)
	require.NoError(t, err)
	require.NotNil(t, identity)
	assert.Equal(t, int64(41), identity.StudentID)
	assert.Equal(t, int64(23), identity.SubmittedBy)
	source.row = nil
	source.err = errors.New("identity failed")
	identity, err = reader.FindByID(ctx, 61)
	assert.Nil(t, identity)
	assert.ErrorIs(t, err, source.err)
	source.err = nil
	identity, err = reader.FindByID(ctx, 61)
	require.NoError(t, err)
	assert.Nil(t, identity)
}
