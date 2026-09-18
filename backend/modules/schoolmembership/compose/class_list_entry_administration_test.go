package compose

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The audited class-list administration (#2382, #3355). The two consumer-owned
// ports are recorded here instead of composed: the student directory and the
// audit command belong to other owners, and what this suite pins is the
// owner's own behaviour — which guard refuses what, and that the trail is
// written in the same transaction as the row.

type recordingStudents struct {
	// byName maps "first|last|class" to the enrolled students carrying it.
	byName map[string][]int64
	// enrolled lists the students that are still enrolled.
	enrolled map[int64]bool
	err      error
}

func (s *recordingStudents) ListStudentIDsByNameAndClass(_ context.Context, firstName, lastName, schoolClass string) ([]int64, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.byName[firstName+"|"+lastName+"|"+schoolClass], nil
}

func (s *recordingStudents) IsEnrolledStudent(_ context.Context, studentID int64) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.enrolled[studentID], nil
}

type recordingTrail struct {
	changes []schoolmembership.ClassListEntryChange
	err     error
}

func (t *recordingTrail) AppendClassListEntryChange(_ context.Context, change schoolmembership.ClassListEntryChange) error {
	if t.err != nil {
		return t.err
	}
	t.changes = append(t.changes, change)
	return nil
}

func buildAdministeredModule(t *testing.T, db *bun.DB) (*schoolmembership.Module, *recordingStudents, *recordingTrail) {
	t.Helper()
	module := buildModule(t, db)
	students := &recordingStudents{byName: map[string][]int64{}, enrolled: map[int64]bool{}}
	trail := &recordingTrail{}
	module.BindClassListEntryAdministration(students, trail)
	return module, students, trail
}

func TestAdministrationRunsTheAuditedLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, _, trail := buildAdministeredModule(t, db)
	ctx := testpkg.Ctx(t)
	actor := testpkg.CreateTestAccount(t, db, "cle-admin-actor@test.local")

	created, err := module.AddClassListEntry(ctx, schoolmembership.AddClassListEntry{
		ClassListEntryFields: schoolmembership.ClassListEntryFields{FirstName: " Zoe ", LastName: "Aalders", SchoolClass: " 7z "},
		ChangedBy:            actor.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, "Zoe", created.FirstName, "the facade trims before persisting")
	require.NotNil(t, created.CreatedBy)
	assert.Equal(t, actor.ID, *created.CreatedBy)

	revised, err := module.ReviseClassListEntry(ctx, schoolmembership.ReviseClassListEntry{
		ID:                   created.ID,
		ClassListEntryFields: schoolmembership.ClassListEntryFields{FirstName: "Zoe", LastName: "Aalders", SchoolClass: "7z-b"},
		ChangedBy:            actor.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, "7z-b", revised.SchoolClass)
	assert.Equal(t, created.CreatedBy, revised.CreatedBy, "a revision never touches the creating account")

	require.NoError(t, module.RemoveClassListEntry(ctx, schoolmembership.RemoveClassListEntry{ID: created.ID, ChangedBy: actor.ID}))
	_, err = module.FindClassListEntry(ctx, created.ID)
	require.ErrorIs(t, err, schoolmembership.ErrClassListEntryNotFound)

	require.Len(t, trail.changes, 3)
	assert.Equal(t, schoolmembership.ClassListEntryChange{
		EntryID: created.ID, Action: "created", NewValue: "Zoe Aalders (7z)", ChangedBy: actor.ID,
	}, trail.changes[0])
	assert.Equal(t, schoolmembership.ClassListEntryChange{
		EntryID: created.ID, Action: "updated", OldValue: "Zoe Aalders (7z)", NewValue: "Zoe Aalders (7z-b)", ChangedBy: actor.ID,
	}, trail.changes[1])
	assert.Equal(t, schoolmembership.ClassListEntryChange{
		EntryID: created.ID, Action: "deleted", OldValue: "Zoe Aalders (7z-b)", ChangedBy: actor.ID,
	}, trail.changes[2])
}

func TestAdministrationRefusesADuplicateEntryOrAnExistingStudent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, students, trail := buildAdministeredModule(t, db)
	ctx := testpkg.Ctx(t)
	actor := testpkg.CreateTestAccount(t, db, "cle-admin-dup@test.local")

	input := schoolmembership.AddClassListEntry{
		ClassListEntryFields: schoolmembership.ClassListEntryFields{FirstName: "Zoe", LastName: "Aalders", SchoolClass: "8a"},
		ChangedBy:            actor.ID,
	}
	first, err := module.AddClassListEntry(ctx, input)
	require.NoError(t, err)

	// The case-folded name is what the class list compares, so a differently
	// spelled duplicate must lose too.
	cased := input
	cased.FirstName = "zoe"
	_, err = module.AddClassListEntry(ctx, cased)
	require.ErrorIs(t, err, schoolmembership.ErrClassListEntryDuplicate)

	students.byName["Ben|Berg|8a"] = []int64{4711}
	_, err = module.AddClassListEntry(ctx, schoolmembership.AddClassListEntry{
		ClassListEntryFields: schoolmembership.ClassListEntryFields{FirstName: "Ben", LastName: "Berg", SchoolClass: "8a"},
		ChangedBy:            actor.ID,
	})
	require.ErrorIs(t, err, schoolmembership.ErrClassListEntryStudentExists)

	// Moving an entry onto a taken name is refused just as well.
	second, err := module.AddClassListEntry(ctx, schoolmembership.AddClassListEntry{
		ClassListEntryFields: schoolmembership.ClassListEntryFields{FirstName: "Ida", LastName: "Ihle", SchoolClass: "8a"},
		ChangedBy:            actor.ID,
	})
	require.NoError(t, err)
	_, err = module.ReviseClassListEntry(ctx, schoolmembership.ReviseClassListEntry{
		ID:                   second.ID,
		ClassListEntryFields: schoolmembership.ClassListEntryFields{FirstName: "Zoe", LastName: "Aalders", SchoolClass: "8a"},
		ChangedBy:            actor.ID,
	})
	require.ErrorIs(t, err, schoolmembership.ErrClassListEntryDuplicate)

	listed, err := module.ListClassListEntries(ctx, schoolmembership.ClassListEntryFilter{})
	require.NoError(t, err)
	require.Len(t, listed, 2, "no refused write may have landed")
	assert.Len(t, trail.changes, 2, "a refused write leaves no trace")
	assert.Equal(t, first.ID, trail.changes[0].EntryID)
}

// Renaming an entry to the value it already carries writes nothing: the
// screen's "save" on an untouched form must not create audit noise, and the
// duplicate guard must not refuse the entry against itself.
func TestRevisionWithoutAChangeIsANoOp(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, _, trail := buildAdministeredModule(t, db)
	ctx := testpkg.Ctx(t)
	actor := testpkg.CreateTestAccount(t, db, "cle-admin-noop@test.local")

	created, err := module.AddClassListEntry(ctx, schoolmembership.AddClassListEntry{
		ClassListEntryFields: schoolmembership.ClassListEntryFields{FirstName: "Zoe", LastName: "Aalders", SchoolClass: "9a"},
		ChangedBy:            actor.ID,
	})
	require.NoError(t, err)

	unchanged, err := module.ReviseClassListEntry(ctx, schoolmembership.ReviseClassListEntry{
		ID:                   created.ID,
		ClassListEntryFields: schoolmembership.ClassListEntryFields{FirstName: " Zoe ", LastName: "Aalders", SchoolClass: "9a"},
		ChangedBy:            actor.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, created, unchanged)
	assert.Len(t, trail.changes, 1, "only the create is on the trail")
}

func TestAdministrationRefusesMissingEntries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, students, _ := buildAdministeredModule(t, db)
	ctx := testpkg.Ctx(t)
	actor := testpkg.CreateTestAccount(t, db, "cle-admin-missing@test.local")
	students.enrolled[4711] = true

	const missing = int64(987654321)
	_, err := module.ReviseClassListEntry(ctx, schoolmembership.ReviseClassListEntry{
		ID:                   missing,
		ClassListEntryFields: schoolmembership.ClassListEntryFields{FirstName: "Zoe", LastName: "Aalders", SchoolClass: "1a"},
		ChangedBy:            actor.ID,
	})
	require.ErrorIs(t, err, schoolmembership.ErrClassListEntryNotFound)
	require.ErrorIs(t,
		module.RemoveClassListEntry(ctx, schoolmembership.RemoveClassListEntry{ID: missing, ChangedBy: actor.ID}),
		schoolmembership.ErrClassListEntryNotFound)
	require.ErrorIs(t,
		module.ResolveClassListEntry(ctx, schoolmembership.ResolveClassListEntry{ID: missing, StudentID: 4711, ChangedBy: actor.ID}),
		schoolmembership.ErrClassListEntryNotFound)
}

// Resolving an entry into a student is an explicit admin action and only ever
// legal when the target IS the child the entry names (#2382: gleichnamige
// Kinder dürfen nicht verwechselt werden).
func TestResolveRequiresTheNamedStudent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, students, trail := buildAdministeredModule(t, db)
	ctx := testpkg.Ctx(t)
	actor := testpkg.CreateTestAccount(t, db, "cle-admin-resolve@test.local")

	created, err := module.AddClassListEntry(ctx, schoolmembership.AddClassListEntry{
		ClassListEntryFields: schoolmembership.ClassListEntryFields{FirstName: "Zoe", LastName: "Aalders", SchoolClass: "4c"},
		ChangedBy:            actor.ID,
	})
	require.NoError(t, err)

	const namesake, stranger = int64(4711), int64(4712)
	require.ErrorIs(t,
		module.ResolveClassListEntry(ctx, schoolmembership.ResolveClassListEntry{ID: created.ID, StudentID: namesake, ChangedBy: actor.ID}),
		schoolmembership.ErrClassListEntryStudentNotFound)

	students.enrolled[namesake] = true
	students.enrolled[stranger] = true
	students.byName["Zoe|Aalders|4c"] = []int64{namesake}

	require.ErrorIs(t,
		module.ResolveClassListEntry(ctx, schoolmembership.ResolveClassListEntry{ID: created.ID, StudentID: stranger, ChangedBy: actor.ID}),
		schoolmembership.ErrClassListEntryAssignMismatch)

	listed, err := module.ListClassListEntries(ctx, schoolmembership.ClassListEntryFilter{})
	require.NoError(t, err)
	require.Len(t, listed, 1, "a refused resolution keeps the entry")

	require.NoError(t, module.ResolveClassListEntry(ctx, schoolmembership.ResolveClassListEntry{
		ID: created.ID, StudentID: namesake, ChangedBy: actor.ID,
	}))
	_, err = module.FindClassListEntry(ctx, created.ID)
	require.ErrorIs(t, err, schoolmembership.ErrClassListEntryNotFound)

	require.Len(t, trail.changes, 2)
	assigned := trail.changes[1]
	assert.Equal(t, "assigned", assigned.Action)
	assert.Equal(t, "Zoe Aalders (4c)", assigned.OldValue)
	require.NotNil(t, assigned.MatchedStudentID)
	assert.Equal(t, namesake, *assigned.MatchedStudentID)
}

// The trail is written in the producer's transaction, so a refused append
// must take the entry down with it — an unaudited change to a child's name is
// exactly what #2382 forbids.
func TestARefusedTrailRollsTheWriteBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, _, trail := buildAdministeredModule(t, db)
	ctx := testpkg.Ctx(t)
	actor := testpkg.CreateTestAccount(t, db, "cle-admin-trail@test.local")
	trail.err = errors.New("audit refused")

	_, err := module.AddClassListEntry(ctx, schoolmembership.AddClassListEntry{
		ClassListEntryFields: schoolmembership.ClassListEntryFields{FirstName: "Zoe", LastName: "Aalders", SchoolClass: "5d"},
		ChangedBy:            actor.ID,
	})
	require.ErrorIs(t, err, trail.err)

	listed, err := module.ListClassListEntries(ctx, schoolmembership.ClassListEntryFilter{})
	require.NoError(t, err)
	assert.Empty(t, listed, "the entry must not survive its refused audit row")
}

func TestDisplayOrderAndMatchHint(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, students, _ := buildAdministeredModule(t, db)
	ctx := testpkg.Ctx(t)

	zander := createEntry(t, ctx, module, "Zoe", "Zander", "2a")
	aerger := createEntry(t, ctx, module, "Anna", "Ärger", "2a")
	berg := createEntry(t, ctx, module, "Ben", "Berg", "10a")

	ordered, err := module.ListClassListEntriesInDisplayOrder(ctx, schoolmembership.ClassListEntryFilter{})
	require.NoError(t, err)
	require.Len(t, ordered, 3)
	assert.Equal(t, []int64{aerger.ID, zander.ID, berg.ID}, []int64{ordered[0].ID, ordered[1].ID, ordered[2].ID},
		"Ä sorts with A, and grade 2 before grade 10")

	students.byName["Ben|Berg|10a"] = []int64{4711}
	matches, err := module.MatchingStudentIDs(ctx, schoolmembership.ClassListEntryFields{
		FirstName: berg.FirstName, LastName: berg.LastName, SchoolClass: berg.SchoolClass,
	})
	require.NoError(t, err)
	assert.Equal(t, []int64{4711}, matches)

	none, err := module.MatchingStudentIDs(ctx, schoolmembership.ClassListEntryFields{
		FirstName: zander.FirstName, LastName: zander.LastName, SchoolClass: zander.SchoolClass,
	})
	require.NoError(t, err)
	assert.Empty(t, none, "no namesake is an empty list, never an error")

	// The hint reads whatever the rows carry. A row with an empty field must
	// match nobody, not fail the listing it is part of.
	incomplete, err := module.MatchingStudentIDs(ctx, schoolmembership.ClassListEntryFields{
		FirstName: "Ben", LastName: "Berg", SchoolClass: " ",
	})
	require.NoError(t, err)
	assert.Empty(t, incomplete)
}

// A module the root never handed the two ports refuses the administration
// commands instead of silently skipping the guards or the trail.
func TestUnboundAdministrationRefusesEveryCommand(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	fields := schoolmembership.ClassListEntryFields{FirstName: "Zoe", LastName: "Aalders", SchoolClass: "1a"}

	_, err := module.AddClassListEntry(ctx, schoolmembership.AddClassListEntry{ClassListEntryFields: fields})
	require.Error(t, err)
	_, err = module.ReviseClassListEntry(ctx, schoolmembership.ReviseClassListEntry{ID: 1, ClassListEntryFields: fields})
	require.Error(t, err)
	require.Error(t, module.RemoveClassListEntry(ctx, schoolmembership.RemoveClassListEntry{ID: 1}))
	require.Error(t, module.ResolveClassListEntry(ctx, schoolmembership.ResolveClassListEntry{ID: 1, StudentID: 2}))
	_, err = module.MatchingStudentIDs(ctx, fields)
	require.Error(t, err)
}
