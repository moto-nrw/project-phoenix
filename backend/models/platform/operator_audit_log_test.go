package platform

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperatorAuditLog_SetChanges_Valid(t *testing.T) {
	log := &OperatorAuditLog{}
	changes := map[string]any{
		"field1": "value1",
		"field2": 123,
		"field3": true,
	}

	err := log.SetChanges(changes)
	require.NoError(t, err)
	assert.NotNil(t, log.Changes)

	// Verify JSON is valid
	retrieved, err := log.GetChanges()
	require.NoError(t, err)
	assert.Equal(t, "value1", retrieved["field1"])
	assert.Equal(t, float64(123), retrieved["field2"]) // JSON unmarshals numbers as float64
	assert.Equal(t, true, retrieved["field3"])
}

func TestOperatorAuditLog_SetChanges_EmptyMap(t *testing.T) {
	log := &OperatorAuditLog{}
	changes := map[string]any{}

	err := log.SetChanges(changes)
	require.NoError(t, err)
	assert.NotNil(t, log.Changes)

	retrieved, err := log.GetChanges()
	require.NoError(t, err)
	assert.Empty(t, retrieved)
}

func TestOperatorAuditLog_GetChanges_Nil(t *testing.T) {
	log := &OperatorAuditLog{
		Changes: nil,
	}

	changes, err := log.GetChanges()
	require.NoError(t, err)
	assert.Nil(t, changes)
}

func TestOperatorAuditLog_GetChanges_Valid(t *testing.T) {
	log := &OperatorAuditLog{}
	original := map[string]any{
		"status": "updated",
		"count":  42,
	}

	err := log.SetChanges(original)
	require.NoError(t, err)

	retrieved, err := log.GetChanges()
	require.NoError(t, err)
	assert.Equal(t, "updated", retrieved["status"])
	assert.Equal(t, float64(42), retrieved["count"])
}

func TestOperatorAuditLog_GetChanges_InvalidJSON(t *testing.T) {
	log := &OperatorAuditLog{
		Changes: []byte(`{invalid json`),
	}

	changes, err := log.GetChanges()
	assert.Error(t, err)
	assert.Nil(t, changes)
}

func TestOperatorAuditLog_TableName(t *testing.T) {
	log := &OperatorAuditLog{}
	assert.Equal(t, "platform.operator_audit_log", log.TableName())
}
