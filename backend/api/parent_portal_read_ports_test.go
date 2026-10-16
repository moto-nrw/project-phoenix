package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
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
}
