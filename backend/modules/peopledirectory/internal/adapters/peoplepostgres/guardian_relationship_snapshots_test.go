package users

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	models "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
	"github.com/stretchr/testify/require"
)

func TestGuardianRelationshipSnapshotRetainsCompleteRead(t *testing.T) {
	t.Parallel()
	text := "retained"
	stamp := time.Date(2026, time.October, 17, 1, 2, 3, 4, time.UTC)
	row := &models.StudentGuardian{StudentID: 23, GuardianProfileID: 29, RelationshipType: "relative", GuardianRole: "pickup_only", IsPrimary: true, IsEmergencyContact: true, CanPickup: true, PickupNotes: &text, EmergencyPriority: 3, IsPayer: true, Permissions: map[string]any{"true": true, "false": false, "null": nil, "number": json.Number("9007199254740993"), "empty": []any{}}, Student: &models.Student{PersonID: 37, SchoolClass: "2a", HealthInfo: &text}}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = 41, 43, stamp, stamp
	row.Student.ID, row.Student.TenantID = row.StudentID, row.TenantID
	got, err := nativeGuardianRelationship(row)
	require.NoError(t, err)
	original, native := reflect.ValueOf(row).Elem(), reflect.ValueOf(got).Elem()
	for i := 0; i < native.NumField(); i++ {
		name := native.Type().Field(i).Name
		if name == "Student" {
			continue
		}
		wantJSON, marshalErr := json.Marshal(original.FieldByName(name).Interface())
		require.NoError(t, marshalErr)
		gotJSON, marshalErr := json.Marshal(native.Field(i).Interface())
		require.NoError(t, marshalErr)
		require.JSONEq(t, string(wantJSON), string(gotJSON), name)
	}
	require.NotNil(t, got.Student)
	require.Equal(t, row.StudentID, got.Student.ID)
	require.Equal(t, row.Student.PersonID, got.Student.PersonID)
	require.Equal(t, row.Student.HealthInfo, got.Student.HealthInfo)
	missing, err := nativeGuardianRelationship(nil)
	require.NoError(t, err)
	require.Nil(t, missing)
	row.Permissions = nil
	row.Student = nil
	got, err = nativeGuardianRelationship(row)
	require.NoError(t, err)
	require.Nil(t, got.Permissions)
	require.Nil(t, got.Student)
}
func TestRetainedAndNativeRelationshipValidationSharePartialFailure(t *testing.T) {
	t.Parallel()
	require.Same(t, models.ErrGuardianProfileNotFound, directory.ErrGuardianProfileNotFound)
	require.Same(t, models.ErrStudentGuardianNotFound, directory.ErrStudentGuardianNotFound)
	for _, relation := range []string{"UNKNOWN ", " PARENT ", "RELATIVE"} {
		t.Run(relation, func(t *testing.T) {
			t.Parallel()
			row := &models.StudentGuardian{StudentID: 23, GuardianProfileID: 29, RelationshipType: relation, GuardianRole: " LEGAL_GUARDIAN "}
			value, err := nativeGuardianRelationship(row)
			require.NoError(t, err)
			oldErr := row.Validate()
			nativeErr := directory.NormalizeGuardianRelationship(value)
			if oldErr == nil {
				require.NoError(t, nativeErr)
			} else {
				require.EqualError(t, nativeErr, oldErr.Error())
			}
			require.Equal(t, row.RelationshipType, value.RelationshipType)
			require.Equal(t, row.GuardianRole, value.GuardianRole)
			require.Equal(t, row.Permissions != nil, value.Permissions != nil)
		})
	}
}
