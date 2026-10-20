package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The five storage moves of #2580 (staff, student, guardian, request-child,
// presence) end in a Contract that drops the old storage. The retention guard
// sees only base tables; this test also covers the views, sequences and the
// presence column the Contracts removed.
func TestStorageContractsLeaveNoRetiredStorage(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	for _, version := range []string{"001015399", "001015414", "001015427", "001015429", "001015432"} {
		var applied bool
		require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM public.bun_migrations WHERE name = ?)`, version).Scan(t.Context(), &applied))
		require.True(t, applied, "contract %s", version)
	}
	for _, relation := range []string{
		"users.staff", "users.staff_legacy", "users.staff_compatibility_reads",
		"users.students", "users.students_legacy", "users.expired_privacy_consents",
		"users.student_compatibility_reads", "users.student_compatibility_writes",
		"users.students_guardians", "users.students_guardians_compatibility_writes",
		"enrollment.request_child_offerings", "enrollment.request_child_offerings_legacy", "enrollment.request_child_compatibility_reads",
		"enrollment.request_child_compatibility_writes",
		"active.presence_compatibility_writes",
	} {
		var present bool
		require.NoError(t, db.NewRaw(`SELECT to_regclass(?) IS NOT NULL`, relation).Scan(t.Context(), &present))
		require.False(t, present, "%s must be gone after the Contract", relation)
	}
	var rebound bool
	require.NoError(t, db.NewRaw(`SELECT
		position('users.students' in pg_get_functiondef('active.count_student_visits_for_deletion(bigint,bigint)'::regprocedure)) = 0
		AND position('users.student_profiles' in pg_get_functiondef('active.count_student_visits_for_deletion(bigint,bigint)'::regprocedure)) > 0`).Scan(t.Context(), &rebound))
	require.True(t, rebound, "the deletion workflow must no longer depend on the removed view")
	var checkedInAt bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT FROM pg_attribute WHERE attrelid = 'schedule.instance_students'::regclass
		AND attname = 'checked_in_at' AND NOT attisdropped)`).Scan(t.Context(), &checkedInAt))
	require.False(t, checkedInAt, "the Presence Contract drops schedule.instance_students.checked_in_at")
}
