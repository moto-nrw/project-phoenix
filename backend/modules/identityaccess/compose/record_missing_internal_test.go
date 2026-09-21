package compose

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The account routes test ErrRecordMissing where they tested the driver's
// no-rows error; the store's text and chain stay intact (#2736).
func TestRecordMissingMarksNoRowsAndKeepsTheStoreText(t *testing.T) {
	t.Parallel()

	stored := fmt.Errorf("find role: %w", sql.ErrNoRows)
	marked := recordMissing(stored)

	require.ErrorIs(t, marked, identityaccess.ErrRecordMissing)
	require.ErrorIs(t, marked, sql.ErrNoRows)
	assert.Equal(t, stored.Error(), marked.Error())
	assert.Same(t, marked, recordMissing(marked), "an already marked error is not wrapped twice")
}

func TestRecordMissingLeavesOtherErrorsAlone(t *testing.T) {
	t.Parallel()

	other := errors.New("connection refused")
	assert.Same(t, other, recordMissing(other))
	assert.NoError(t, recordMissing(nil))
}

func TestRoleAndPasswordResetErrorsMarkNoRows(t *testing.T) {
	t.Parallel()

	for name, translate := range map[string]func(error) error{
		"role":           roleError,
		"password reset": passwordResetError,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			marked := translate(fmt.Errorf("update: %w", sql.ErrNoRows))
			require.ErrorIs(t, marked, identityaccess.ErrRecordMissing)
			require.ErrorIs(t, marked, sql.ErrNoRows)
		})
	}
}
