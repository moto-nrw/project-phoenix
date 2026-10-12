package compose

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStudentExternalIDNamesOneChild covers the key another system knows a
// child by (#1446): the directory finds the child by it, an active child
// keeps its key, and a graduate gives it up to the child now known by it.
func TestStudentExternalIDNamesOneChild(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := New(Dependencies{DB: db, Observe: func(Observation) {}})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	first := testpkg.CreateTestStudent(t, db, "Key", "First", "1a")
	second := testpkg.CreateTestStudent(t, db, "Key", "Second", "1a")
	key := fmt.Sprintf("iserv:%d", time.Now().UnixNano())
	byKey := peopledirectory.StudentDirectoryFilter{ExternalIDs: []string{" " + key + " "}, CareStatus: peopledirectory.StudentCareStatusAll}

	require.NoError(t, module.SetStudentExternalID(ctx, first.ID, key))
	require.NoError(t, module.SetStudentExternalID(ctx, first.ID, key), "setting the held key again is no change")
	found, err := module.ListStudentDirectory(ctx, byKey)
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, first.ID, found[0].ID)

	err = module.SetStudentExternalID(ctx, second.ID, key)
	require.ErrorIs(t, err, peopledirectory.ErrStudentExternalIDTaken, "an active child keeps its key")

	_, err = db.NewRaw(`UPDATE users.student_school_memberships SET status = ? WHERE student_profile_id = ?`,
		peopledirectory.StudentStatusAlumnus, first.ID).Exec(ctx)
	require.NoError(t, err)
	found, err = module.ListStudentDirectory(ctx, byKey)
	require.NoError(t, err)
	assert.Empty(t, found, "the directory leaves a graduate out")

	require.NoError(t, module.SetStudentExternalID(ctx, second.ID, key), "a graduate gives the key up")
	found, err = module.ListStudentDirectory(ctx, byKey)
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, second.ID, found[0].ID)

	var firstKey *string
	require.NoError(t, db.NewRaw(`SELECT external_id FROM users.student_profiles WHERE id = ?`, first.ID).Scan(ctx, &firstKey))
	assert.Nil(t, firstKey)
}
