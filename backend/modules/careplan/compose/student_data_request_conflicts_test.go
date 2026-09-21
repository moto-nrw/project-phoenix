package compose

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// driverError is the pgdriver shape; the raw database error must survive the
// sentinel so callers can still log or inspect it.
type driverError interface {
	error
	Field(byte) string
}

func TestStudentDataRequestDuplicatePendingFieldReturnsSentinel(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "parent")
	student := testpkg.CreateTestStudent(t, db, "Pending", "Field", "1a")
	request := func(field string) careplan.StudentDataChangeRequest {
		return careplan.StudentDataChangeRequest{
			StudentID: student.ID, SubmittedBy: account.ID, Target: "person", FieldKey: field,
			OldValue: json.RawMessage(`"Alt"`), NewValue: json.RawMessage(`"Neu"`), Status: "pending",
		}
	}
	_, err := module.CreateStudentDataRequest(ctx, request("first_name"))
	require.NoError(t, err)

	_, err = module.CreateStudentDataRequest(ctx, request("first_name"))
	require.ErrorIs(t, err, careplan.ErrStudentDataRequestFieldPending)
	var raw driverError
	require.True(t, errors.As(err, &raw))
	assert.Equal(t, "23505", raw.Field('C'))
	assert.Equal(t, "uniq_student_data_change_requests_pending_field", raw.Field('n'))

	_, err = module.CreateStudentDataRequest(ctx, request("last_name"))
	require.NoError(t, err, "another field is not a duplicate")
}
