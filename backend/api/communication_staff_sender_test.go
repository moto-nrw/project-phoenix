package api

import (
	"context"
	"errors"
	"testing"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type staffPersonReader func(context.Context, int64) (*peopleValues.DirectoryPersonSnapshot, error)

func (read staffPersonReader) FindByAccountID(ctx context.Context, id int64) (*peopleValues.DirectoryPersonSnapshot, error) {
	return read(ctx, id)
}

func TestStaffSenderBindingPreservesContextNamePartsAndPartialErrors(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	failure := errors.New("partial person lookup")
	calls := 0
	source := staffPersonReader(func(actual context.Context, id int64) (*peopleValues.DirectoryPersonSnapshot, error) {
		calls++
		assert.Same(t, ctx, actual)
		assert.EqualValues(t, 13, id)
		return &peopleValues.DirectoryPersonSnapshot{FirstName: "  Anna ", LastName: " Berg  "}, failure
	})
	row, err := owners.BindStaffSenderNames(source).FindStaffSenderName(ctx, 13)
	assert.Same(t, failure, err)
	require.NotNil(t, row)
	assert.Equal(t, "  Anna ", row.FirstName)
	assert.Equal(t, " Berg  ", row.LastName)
	assert.Equal(t, 1, calls, "name projection must not perform another lookup")
	missing := owners.BindStaffSenderNames(staffPersonReader(func(context.Context, int64) (*peopleValues.DirectoryPersonSnapshot, error) { return nil, failure }))
	row, err = missing.FindStaffSenderName(ctx, 13)
	assert.Nil(t, row)
	assert.Same(t, failure, err)
	assert.Nil(t, owners.BindStaffSenderNames(nil))
}
