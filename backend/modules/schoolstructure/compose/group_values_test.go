package compose

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/stretchr/testify/require"
)

func TestNativeGroupStorageErrorsPreserveTextAndClassification(t *testing.T) {
	t.Parallel()
	for _, cause := range []error{legacyGroupRecordTestNotFound, errors.New("driver failure")} {
		original := &EducationError{Op: "CountGroups", Err: &legacyGroupStorageTestError{Op: "count", Err: cause}}
		native := publicGroupError(original)
		require.Equal(t, original.Error(), native.Error())
		var service *schoolstructure.EducationError
		require.ErrorAs(t, native, &service)
		var legacyService *EducationError
		require.False(t, errors.As(native, &legacyService))
		var legacyStorage *legacyGroupStorageTestError
		require.False(t, errors.As(native, &legacyStorage))
		var storage interface{ StoreFailure() bool }
		require.ErrorAs(t, native, &storage)
		require.True(t, storage.StoreFailure())
		var missing interface{ RepositoryNotFound() }
		require.Equal(t, cause == legacyGroupRecordTestNotFound, errors.As(native, &missing))
		if cause != legacyGroupRecordTestNotFound {
			require.ErrorIs(t, native, cause)
		}
		require.ErrorIs(t, legacyGroupError(native), cause, "retained suite adapter preserves original error identity")
	}
}

func TestNativeGroupErrorsPreserveWrappedAndJoinedCauses(t *testing.T) {
	t.Parallel()
	sibling := errors.New("another failure")
	original := fmt.Errorf("outer context: %w", errors.Join(
		&EducationError{Op: "GetGroup", Err: &legacyGroupStorageTestError{Op: "find by id", Err: errors.Join(legacyGroupRecordTestNotFound, sql.ErrNoRows)}},
		sibling,
	))
	native := publicGroupError(original)
	require.Equal(t, original.Error(), native.Error())
	require.ErrorIs(t, native, sibling)
	require.ErrorIs(t, native, sql.ErrNoRows)
	require.NotErrorIs(t, native, legacyGroupRecordTestNotFound)
	var missing interface{ RepositoryNotFound() }
	require.ErrorAs(t, native, &missing)
	var legacyStorage *legacyGroupStorageTestError
	require.False(t, errors.As(native, &legacyStorage))
	var legacyService *EducationError
	require.False(t, errors.As(native, &legacyService))
	retained := legacyGroupError(native)
	require.Equal(t, original.Error(), retained.Error())
	require.ErrorIs(t, retained, sibling)
	require.ErrorIs(t, retained, sql.ErrNoRows)
	require.ErrorIs(t, retained, legacyGroupRecordTestNotFound)
}
