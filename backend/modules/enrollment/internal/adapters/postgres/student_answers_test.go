package postgres_test

import (
	"context"
	"testing"

	capability "github.com/moto-ogs/project-phoenix/modules/enrollment"
	ownerCompose "github.com/moto-ogs/project-phoenix/modules/enrollment/compose"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// TestStudentAnswersReadOnlyChildrenTheApprovalLinked reads the answers the
// child's page reads: linked by created_student_id, never an office match,
// never another school's.
func TestStudentAnswersReadOnlyChildrenTheApprovalLinked(t *testing.T) {
	t.Parallel()
	db, owner, tenantID, _, requestID := setupRequestChildRepoTest(t)
	creator := testpkg.CreateTestAccount(t, db, "studentanswers")
	schema := &capability.FormSchema{Name: uniqueSchemaName("answers"), Version: 1, IsActive: true, CreatedBy: creator.ID,
		Fields: []capability.FormField{{Key: "seepferdchen", Label: "Seepferdchen vorhanden", Type: capability.FormFieldBoolean, AppliesToCh: true}}}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error { return owner.InsertSchemaVersion(ctx, schema) }))
	_, err := db.ExecContext(testpkg.Ctx(t), `UPDATE enrollment.requests SET schema_id = ? WHERE id = ?`, schema.ID, requestID)
	require.NoError(t, err)

	linked := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Mila", "Sommer", "1a")
	matched := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Noah", "Sommer", "3b")
	unlinked := makeChild(requestID, "Ida", "Sommer")
	for _, child := range []struct {
		row     *capability.RequestChild
		student int64
		office  bool
	}{{makeChild(requestID, "Mila", "Sommer"), linked.ID, false}, {makeChild(requestID, "Noah", "Sommer"), matched.ID, true}, {unlinked, 0, false}} {
		child.row.CustomData.Set("seepferdchen", true)
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			if err := owner.InsertChild(ctx, child.row); err != nil {
				return err
			}
			switch {
			case child.office:
				return owner.PinOfficeChosenStudent(ctx, child.row.ID, child.student)
			case child.student > 0:
				return owner.LinkCreatedStudent(ctx, child.row.ID, child.student)
			}
			return nil
		}))
	}

	reads := ownerCompose.NewStudentAnswers()
	var ids []int64
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		ids, err = reads.StudentsWithAnswer(ctx, "seepferdchen", "true")
		return err
	}))
	require.Equal(t, []int64{linked.ID}, ids)

	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	require.NoError(t, runInTenantTx(t, db, otherTenant, func(ctx context.Context) error {
		ids, err = reads.StudentsWithAnswer(ctx, "seepferdchen", "true")
		return err
	}))
	require.Empty(t, ids, "another school reads none of these answers")
}
