package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	parentModels "github.com/moto-nrw/project-phoenix/modules/careplan/internal/rows"
)

type parentChildQuerySource struct {
	row                *parentModels.ChildSummary
	rows               []*parentModels.ChildSummary
	err                error
	observe            func(context.Context)
	accountID, childID int64
}

func (s *parentChildQuerySource) FindForAccount(ctx context.Context, accountID, childID int64) (*parentModels.ChildSummary, error) {
	if s.observe != nil {
		s.observe(ctx)
	}
	s.accountID, s.childID = accountID, childID
	return s.row, s.err
}

func (s *parentChildQuerySource) ListByAccount(ctx context.Context, accountID int64) ([]*parentModels.ChildSummary, error) {
	if s.observe != nil {
		s.observe(ctx)
	}
	s.accountID = accountID
	return s.rows, s.err
}

func TestParentChildProjectionDelegatesStoredPermissions(t *testing.T) {
	t.Parallel()

	from, until := careplan.Date("2026-08-01"), careplan.Date("2027-07-31")
	source := &parentChildQuerySource{row: &parentModels.ChildSummary{
		StudentID: 11, TenantID: 23, PersonID: 31, FirstName: "Alex", LastName: "Test",
		SchoolClass: "3a", Status: "active", EnrolledFrom: &from, EnrolledUntil: &until,
		SchoolName: "School", SchoolSlug: "school", GuardianProfileID: 43,
		GuardianPermissions: map[string]interface{}{"yes": "grant", "true": true, "false": false, "nil": nil, "zero": 0},
	}}
	var seen []any
	queries := NewParentChildQueries(source, func(value any) bool {
		seen = append(seen, value)
		return value == "grant"
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source.observe = func(actual context.Context) { assert.Same(t, ctx, actual) }
	child, err := queries.FindForAccount(ctx, 53, 11)
	require.NoError(t, err)
	require.NotNil(t, child)
	assert.Equal(t, int64(53), source.accountID)
	assert.Equal(t, int64(11), source.childID)
	assert.ElementsMatch(t, []any{"grant", true, false, nil, 0}, seen)
	assert.Equal(t, map[string]bool{"yes": true, "true": false, "false": false, "nil": false, "zero": false}, child.GuardianPermissions)
	// Even true must go through the supplied authority. The projection cannot
	// invent its own grant rule. Non-permission wire fields remain identical.
	before, err := json.Marshal(source.row)
	require.NoError(t, err)
	after, err := json.Marshal(child)
	require.NoError(t, err)
	assert.JSONEq(t, string(before), string(after))

	source.row = nil
	source.err = errors.New("read failed")
	child, err = queries.FindForAccount(ctx, 53, 11)
	assert.Nil(t, child)
	assert.ErrorIs(t, err, source.err)
	rows, err := queries.ListByAccount(ctx, 53)
	assert.Nil(t, rows)
	assert.ErrorIs(t, err, source.err)
	source.err = nil
	source.rows = []*parentModels.ChildSummary{}
	rows, err = queries.ListByAccount(ctx, 53)
	require.NoError(t, err)
	require.NotNil(t, rows)
	assert.Empty(t, rows)
}
