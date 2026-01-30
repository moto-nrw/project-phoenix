package audit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// DataImport Tests
// =============================================================================

func TestDataImport_DefaultValues(t *testing.T) {
	di := DataImport{}

	assert.Zero(t, di.ID)
	assert.Empty(t, di.EntityType)
	assert.Empty(t, di.Filename)
	assert.Zero(t, di.TotalRows)
	assert.Zero(t, di.CreatedCount)
	assert.Zero(t, di.UpdatedCount)
	assert.Zero(t, di.SkippedCount)
	assert.Zero(t, di.ErrorCount)
	assert.Zero(t, di.WarningCount)
	assert.False(t, di.DryRun)
	assert.Zero(t, di.ImportedBy)
	assert.Nil(t, di.CompletedAt)
	assert.Nil(t, di.Metadata)
}

func TestDataImport_WithValues(t *testing.T) {
	now := time.Now()
	completed := now.Add(5 * time.Second)

	di := DataImport{
		ID:           1,
		EntityType:   "student",
		Filename:     "students_2026.csv",
		TotalRows:    100,
		CreatedCount: 80,
		UpdatedCount: 15,
		SkippedCount: 3,
		ErrorCount:   2,
		WarningCount: 5,
		DryRun:       false,
		ImportedBy:   42,
		StartedAt:    now,
		CompletedAt:  &completed,
		Metadata:     JSONBMap{"errors": "details"},
	}

	assert.Equal(t, int64(1), di.ID)
	assert.Equal(t, "student", di.EntityType)
	assert.Equal(t, "students_2026.csv", di.Filename)
	assert.Equal(t, 100, di.TotalRows)
	assert.Equal(t, 80, di.CreatedCount)
	assert.Equal(t, 15, di.UpdatedCount)
	assert.Equal(t, 3, di.SkippedCount)
	assert.Equal(t, 2, di.ErrorCount)
	assert.Equal(t, 5, di.WarningCount)
	assert.False(t, di.DryRun)
	assert.Equal(t, int64(42), di.ImportedBy)
	assert.NotNil(t, di.CompletedAt)
	assert.NotNil(t, di.Metadata)
}

// =============================================================================
// JSONBMap Tests
// =============================================================================

func TestJSONBMap_Assignment(t *testing.T) {
	m := JSONBMap{
		"key1": "value1",
		"key2": 42,
		"key3": true,
	}

	assert.Equal(t, "value1", m["key1"])
	assert.Equal(t, 42, m["key2"])
	assert.Equal(t, true, m["key3"])
}

func TestJSONBMap_NilMap(t *testing.T) {
	var m JSONBMap
	assert.Nil(t, m)
}
