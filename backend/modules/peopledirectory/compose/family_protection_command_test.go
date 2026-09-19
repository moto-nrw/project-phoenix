package compose

import (
	"context"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The cases below moved here with the family-protection application logic
// (#3349); they previously drove services/users.FamilyProtectionService.

func TestSetFamilyProtectionAppendsAuditedStateChange(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := New(Dependencies{DB: db, Observe: func(Observation) {}})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Protection", "Append", "1a")
	account := testpkg.CreateTestAccount(t, db, "family-protection-append@example.test")

	enabled, err := module.SetFamilyProtection(ctx, peopledirectory.SetFamilyProtection{
		StudentID: student.ID, Enabled: true, Reason: "Schutz nötig", ActorAccountID: account.ID,
	})
	require.NoError(t, err)
	require.True(t, enabled)

	var row struct {
		Enabled        bool   `bun:"enabled"`
		Reason         string `bun:"reason"`
		ActorAccountID int64  `bun:"actor_account_id"`
	}
	require.NoError(t, db.NewRaw(
		"SELECT enabled, reason, actor_account_id FROM users.student_family_protection_events WHERE student_id = ? ORDER BY id DESC LIMIT 1",
		student.ID,
	).Scan(ctx, &row))
	require.True(t, row.Enabled)
	require.Equal(t, "Schutz nötig", row.Reason)
	require.Equal(t, account.ID, row.ActorAccountID)

	current, err := module.CurrentFamilyProtection(ctx, []int64{student.ID})
	require.NoError(t, err)
	require.Equal(t, map[int64]bool{student.ID: true}, current)
}

// Repeating a switch is not an error the caller has to fix, but it is not a
// state change either: the sentinel travels with the current state so the API
// can answer 200 and say "unchanged" (#2267).
func TestSetFamilyProtectionDoesNotAppendUnchangedState(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := New(Dependencies{DB: db, Observe: func(Observation) {}})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Protection", "Unchanged", "1a")
	account := testpkg.CreateTestAccount(t, db, "family-protection-unchanged@example.test")

	_, err = module.SetFamilyProtection(ctx, peopledirectory.SetFamilyProtection{
		StudentID: student.ID, Enabled: true, Reason: "Schutz nötig", ActorAccountID: account.ID,
	})
	require.NoError(t, err)

	enabled, err := module.SetFamilyProtection(ctx, peopledirectory.SetFamilyProtection{
		StudentID: student.ID, Enabled: true, Reason: "noch einmal", ActorAccountID: account.ID,
	})
	require.ErrorIs(t, err, peopledirectory.ErrFamilyProtectionUnchanged)
	require.True(t, enabled)

	var count int
	require.NoError(t, db.NewRaw(
		"SELECT count(*) FROM users.student_family_protection_events WHERE student_id = ?", student.ID,
	).Scan(ctx, &count))
	require.Equal(t, 1, count)
}

func TestSetFamilyProtectionRejectsInvalidChanges(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := New(Dependencies{DB: db, Observe: func(Observation) {}})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Protection", "Invalid", "1a")
	account := testpkg.CreateTestAccount(t, db, "family-protection-invalid@example.test")

	cases := map[string]peopledirectory.SetFamilyProtection{
		"missing student": {Enabled: true, Reason: "Schutz", ActorAccountID: account.ID},
		"missing actor":   {StudentID: student.ID, Enabled: true, Reason: "Schutz"},
		"blank reason":    {StudentID: student.ID, Enabled: true, Reason: "   ", ActorAccountID: account.ID},
		"reason too long": {
			StudentID: student.ID, Enabled: true, ActorAccountID: account.ID,
			Reason: strings.Repeat("ä", peopledirectory.MaxFamilyProtectionReasonRunes+1),
		},
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := module.SetFamilyProtection(ctx, input)
			require.ErrorIs(t, err, peopledirectory.ErrFamilyProtectionInvalid)
		})
	}

	var count int
	require.NoError(t, db.NewRaw(
		"SELECT count(*) FROM users.student_family_protection_events WHERE student_id = ?", student.ID,
	).Scan(ctx, &count))
	require.Zero(t, count)
}

// A graduation committing before the flip must not leave an alumnus protected:
// the command locks the row and refuses.
func TestSetFamilyProtectionRefusesAlumnus(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := New(Dependencies{DB: db, Observe: func(Observation) {}})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Protection", "Alumnus", "1a")
	account := testpkg.CreateTestAccount(t, db, "family-protection-alumnus@example.test")
	affected, err := module.GraduateStudents(ctx, []int64{student.ID})
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)

	_, err = module.SetFamilyProtection(ctx, peopledirectory.SetFamilyProtection{
		StudentID: student.ID, Enabled: true, Reason: "Schutz nötig", ActorAccountID: account.ID,
	})
	require.ErrorIs(t, err, peopledirectory.ErrStudentNotFound)
}

func TestSetFamilyProtectionDoesNotWriteAnotherTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := New(Dependencies{DB: db, Observe: func(Observation) {}})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	foreign := testpkg.CreateTestStudentForTenant(t, db, otherTenant, "Foreign", "Protection", "2a")
	account := testpkg.CreateTestAccount(t, db, "family-protection-foreign@example.test")

	_, err = module.SetFamilyProtection(ctx, peopledirectory.SetFamilyProtection{
		StudentID: foreign.ID, Enabled: true, Reason: "Schutz nötig", ActorAccountID: account.ID,
	})
	require.ErrorIs(t, err, peopledirectory.ErrStudentNotFound)

	_, err = module.SetFamilyProtection(context.Background(), peopledirectory.SetFamilyProtection{
		StudentID: foreign.ID, Enabled: true, Reason: "Schutz nötig", ActorAccountID: account.ID,
	})
	require.Error(t, err, "this privacy write must not fall back to an administrative transaction")
}
