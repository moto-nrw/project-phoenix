package test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
)

func issue(c *QueryCounter, sql string) {
	c.BeforeQuery(context.Background(), &bun.QueryEvent{Query: sql})
}

type queryCounterRowResult int64

func (r queryCounterRowResult) RowsAffected() (int64, error) { return int64(r), nil }
func (r queryCounterRowResult) LastInsertId() (int64, error) { return 0, nil }

func TestQueryCounterWriteRowsExcludeReadsAndFailedStatements(t *testing.T) {
	t.Parallel()
	c := NewQueryCounter()
	ctx := context.Background()
	for _, event := range []bun.QueryEvent{
		{Query: "SELECT id FROM enrollment.requests", Result: queryCounterRowResult(100)},
		{Query: "INSERT INTO enrollment.requests DEFAULT VALUES RETURNING id", Result: queryCounterRowResult(1)},
		{Query: "UPDATE enrollment.requests SET status = ?", Result: queryCounterRowResult(2)},
		{Query: "DELETE FROM enrollment.requests WHERE id = ?", Result: queryCounterRowResult(1)},
		{Query: "UPDATE enrollment.requests SET status = ?", Result: queryCounterRowResult(9), Err: errors.New("failed")},
	} {
		c.AfterQuery(ctx, &event)
	}
	var expectedRows int64 = 4
	assert.Equal(t, expectedRows, c.WriteRows())
	c.Stop()
	c.AfterQuery(ctx, &bun.QueryEvent{Query: "DELETE FROM enrollment.requests", Result: queryCounterRowResult(50)})
	assert.Equal(t, expectedRows, c.WriteRows())
	c.Reset()
	assert.Zero(t, c.WriteRows())
}

func TestQueryCounterBucketsAndToggles(t *testing.T) {
	t.Parallel()
	c := NewQueryCounter()
	issue(c, `SELECT * FROM config.setting_values WHERE setting_key = 'x'`)
	issue(c, `SELECT "s".* FROM users.students AS "s"`)
	issue(c, `INSERT INTO users.students (id) VALUES (1)`)

	assert.Equal(t, 3, c.Total())
	assert.Len(t, c.Operation("SELECT"), 2)
	assert.Len(t, c.Selects("users.students"), 1, "the INSERT on the same table must not count as a SELECT")
	assert.Len(t, c.Selects("CONFIG.SETTING_VALUES"), 1, "table match is case-insensitive")
	assert.Empty(t, c.Matching(func(q string) bool { return q == "" }))

	c.Stop()
	issue(c, `SELECT 1`)
	assert.Equal(t, 3, c.Total(), "a stopped counter ignores statements")
	c.Start()
	issue(c, `SELECT 1`)
	assert.Equal(t, 4, c.Total())

	c.Reset()
	assert.Equal(t, 0, c.Total())
	assert.Empty(t, c.Queries())
}
