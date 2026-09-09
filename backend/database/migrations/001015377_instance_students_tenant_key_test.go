package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestInstanceStudentsTenantKeySupportsPresenceFK(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	tx, err := db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(t.Context(), `CREATE TABLE active.presence_fk_probe (
		tenant_id BIGINT NOT NULL, instance_student_id BIGINT NOT NULL,
		FOREIGN KEY (tenant_id, instance_student_id) REFERENCES schedule.instance_students(tenant_id, id)
	)`)
	require.NoError(t, err, "Presence must be able to reference planned participants within the same tenant")
}
