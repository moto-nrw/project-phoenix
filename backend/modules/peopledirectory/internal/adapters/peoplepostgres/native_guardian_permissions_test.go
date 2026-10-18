package users

import (
	"encoding/json"
	"testing"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/stretchr/testify/require"
)

func TestNativeGuardianPermissionsRejectTrailingJSON(t *testing.T) {
	t.Parallel()
	_, err := GuardianRelationshipRow(&directory.GuardianRelationshipSnapshot{Permissions: json.RawMessage(`{"can_manage":false}{"can_manage":true}`)})
	require.Error(t, err)
}

func TestNativeGuardianPermissionsPreserveNumbers(t *testing.T) {
	t.Parallel()
	row, err := GuardianRelationshipRow(&directory.GuardianRelationshipSnapshot{Permissions: json.RawMessage(`{"record_id":9007199254740993,"can_manage":false}`)})
	require.NoError(t, err)
	require.Equal(t, json.Number("9007199254740993"), row.Permissions["record_id"])
	require.JSONEq(t, `{"record_id":9007199254740993,"can_manage":false}`, string(NativeGuardianRelationshipValue(row).Permissions))
}
