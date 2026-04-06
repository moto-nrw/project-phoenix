package audit_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/stretchr/testify/assert"
)

func TestDataAccessLog_TableName(t *testing.T) {
	d := &audit.DataAccessLog{}
	assert.Equal(t, "audit.data_access_log", d.TableName())
}

func TestDataAccessLog_GetID(t *testing.T) {
	d := &audit.DataAccessLog{ID: 42}
	assert.Equal(t, int64(42), d.GetID())
}

func TestDataAccessLog_GetCreatedAt(t *testing.T) {
	now := time.Now()
	d := &audit.DataAccessLog{AccessedAt: now}
	assert.Equal(t, now, d.GetCreatedAt())
}

func TestDataAccessLog_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	d := &audit.DataAccessLog{AccessedAt: now}
	assert.Equal(t, now, d.GetUpdatedAt(), "GetUpdatedAt mirrors AccessedAt for append-only rows")
}
