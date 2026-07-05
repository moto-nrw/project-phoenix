package users

// Tests for the BUN persistence hooks and base accessors on the parent-message
// models: BeforeAppendModel must route Insert/Update/Delete to the right
// schema-qualified table expression, and the base.Model accessors must surface
// the embedded id/timestamps. No DB connection is needed — the queries are built
// against a connectionless bun.DB purely to drive the hooks.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func hookDB() *bun.DB { return bun.NewDB(nil, pgdialect.New()) }

func TestParentMessage_BeforeAppendModelRoutesAllQueryKinds(t *testing.T) {
	db := hookDB()
	m := &ParentMessage{}
	require.NoError(t, m.BeforeAppendModel(db.NewInsert()))
	require.NoError(t, m.BeforeAppendModel(db.NewUpdate()))
	require.NoError(t, m.BeforeAppendModel(db.NewDelete()))
	// A non-query argument is a no-op, not a panic.
	require.NoError(t, m.BeforeAppendModel(nil))
}

func TestParentMessageThread_BeforeAppendModelRoutesAllQueryKinds(t *testing.T) {
	db := hookDB()
	th := &ParentMessageThread{}
	require.NoError(t, th.BeforeAppendModel(db.NewInsert()))
	require.NoError(t, th.BeforeAppendModel(db.NewUpdate()))
	require.NoError(t, th.BeforeAppendModel(db.NewDelete()))
	require.NoError(t, th.BeforeAppendModel(nil))
}

func TestBaseModel_Accessors(t *testing.T) {
	now := time.Now()
	m := base.Model{ID: 42, CreatedAt: now, UpdatedAt: now.Add(time.Hour)}
	assert.Equal(t, int64(42), m.GetID())
	assert.Equal(t, now, m.GetCreatedAt())
	assert.Equal(t, now.Add(time.Hour), m.GetUpdatedAt())
}
