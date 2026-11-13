package compose

import (
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The batched namesake lookup (#5891) must answer every key exactly like the
// single-key lookup: the database's Unicode equality, blank and repeated
// keys, other classes, deleted persons and other tenants included. Only the
// number of statements may differ.
func TestEnrolledStudentIDsByNamesAndClassesAnswersLikeTheSingleKeyLookup(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)

	umlaut := testpkg.CreateTestStudent(t, db, "Jörg", "Größe", "2b")
	decomposed := testpkg.CreateTestStudent(t, db, "Legacy", "Pair", "2b")
	_, err := db.NewRaw(`UPDATE users.persons SET first_name = ?, last_name = ? WHERE id = ?`,
		"Jörg", "Größe", decomposed.PersonID).Exec(ctx)
	require.NoError(t, err)
	ascii := testpkg.CreateTestStudent(t, db, "Jorg", "Grosse", "2b")
	asciiTwin := testpkg.CreateTestStudent(t, db, "  JORG", "grosse ", "2B")
	otherClass := testpkg.CreateTestStudent(t, db, "Jorg", "Grosse", "3c")
	deleted := testpkg.CreateTestStudent(t, db, "Geloescht", "Person", "2b")
	// A deleted person beside a child row: the person deletion refuses it
	// since #6840, so the fixture writes the legacy state directly.
	_, err = db.NewRaw(`UPDATE users.persons SET deleted_at = now() WHERE id = ?`, deleted.PersonID).Exec(ctx)
	require.NoError(t, err)
	testpkg.CreateTestPerson(t, db, "Ohne", "Kind")

	keys := []peopledirectory.NameAndClass{
		{FirstName: " JÖRG ", LastName: "größe", SchoolClass: " 2B "},
		{FirstName: "Jorg", LastName: "Grosse", SchoolClass: "2b"},
		{FirstName: "  ", LastName: "Grosse", SchoolClass: "2b"},
		{FirstName: "Jörg", LastName: "GRÖSSE", SchoolClass: "2b"},
		{FirstName: "jorg", LastName: "GROSSE", SchoolClass: "3c"},
		{FirstName: "Jorg", LastName: "Grosse", SchoolClass: "2b"},
		{FirstName: "Geloescht", LastName: "Person", SchoolClass: "2b"},
		{FirstName: "Ohne", LastName: "Kind", SchoolClass: ""},
		{FirstName: "Niemand", LastName: "Hier", SchoolClass: "2b"},
	}
	assertBatchMatchesSingle(t, module, keys)

	batched, err := peopledirectory.EnrolledStudentIDsByNamesAndClasses(ctx, module, keys)
	require.NoError(t, err)
	assert.ElementsMatch(t, []int64{umlaut.ID, decomposed.ID}, batched[0], "NFC and case equality come from the database")
	assert.ElementsMatch(t, []int64{ascii.ID, asciiTwin.ID}, batched[1], "ASCII keys fold case and trim spaces like the database")
	assert.Empty(t, batched[2], "a blank name matches nobody")
	assert.Equal(t, []int64{otherClass.ID}, batched[4])
	assert.Equal(t, batched[1], batched[5], "a repeated key gets its own answer")
	assert.Empty(t, batched[6], "deleted persons are left out")
	assert.Empty(t, batched[7], "a person without a student is no namesake")

	otherCtx, _ := otherTenantContext(t, db)
	foreign, err := peopledirectory.EnrolledStudentIDsByNamesAndClasses(otherCtx, module, keys[:2])
	require.NoError(t, err)
	assert.Equal(t, [][]int64{nil, nil}, foreign, "another tenant sees none of these students")

	empty, err := peopledirectory.EnrolledStudentIDsByNamesAndClasses(ctx, module, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

// NFC folds the Kelvin sign onto an ASCII K, so the database matches a
// non-ASCII spelling to an ASCII key. The shared read must not guess that
// folding; the scalar reads decide.
func TestEnrolledStudentIDsByNamesAndClassesLeavesUnicodeFoldingToTheDatabase(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)

	plain := testpkg.CreateTestStudent(t, db, "Kai", "Kelvin", "4a")
	kelvin := testpkg.CreateTestStudent(t, db, "Kelvinsign", "Kelvin", "4a")
	_, err := db.NewRaw(`UPDATE users.persons SET first_name = ? WHERE id = ?`, "Kai", kelvin.PersonID).Exec(ctx)
	require.NoError(t, err)
	other := testpkg.CreateTestStudent(t, db, "Ida", "Kelvin", "4a")

	keys := []peopledirectory.NameAndClass{
		{FirstName: "Kai", LastName: "Kelvin", SchoolClass: "4a"},
		{FirstName: "Ida", LastName: "Kelvin", SchoolClass: "4a"},
	}
	assertBatchMatchesSingle(t, module, keys)
	batched, err := peopledirectory.EnrolledStudentIDsByNamesAndClasses(ctx, module, keys)
	require.NoError(t, err)
	assert.ElementsMatch(t, []int64{plain.ID, kelvin.ID}, batched[0])
	assert.Equal(t, []int64{other.ID}, batched[1])
}

func assertBatchMatchesSingle(t *testing.T, module peopledirectory.Owner, keys []peopledirectory.NameAndClass) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	batched, err := peopledirectory.EnrolledStudentIDsByNamesAndClasses(ctx, module, keys)
	require.NoError(t, err)
	require.Len(t, batched, len(keys))
	for i, key := range keys {
		single, err := peopledirectory.EnrolledStudentIDsByNameAndClass(ctx, module, key.FirstName, key.LastName, key.SchoolClass)
		require.NoError(t, err)
		if len(single) == 0 {
			assert.Empty(t, batched[i], "key %d", i)
			continue
		}
		assert.Equal(t, single, batched[i], "key %d answers like the single-key lookup, in its order", i)
	}
}
