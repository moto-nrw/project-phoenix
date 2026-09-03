package test

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/uptrace/bun"
)

// QueryCounter is the shared bun.QueryHook behind every query-budget test
// (#2940). It records each SQL statement issued through the hooked *bun.DB
// (statements inside tenant transactions included) while enabled, so a test
// can bound the total, one operation, or the statements touching one table.
//
// Attach it with CaptureQueries (db.AddQueryHook, disabled again at cleanup)
// or hand NewQueryCounter to db.WithQueryHook for a private bun clone. bun
// never removes hooks, so a counter on a shared package DB keeps seeing every
// later statement; Stop() at cleanup is what keeps tests from counting each
// other. Tests that count must own their database (SetupIsolatedTestDB) or a
// WithQueryHook clone before they run in parallel with other DB tests.
type QueryCounter struct {
	enabled atomic.Bool
	mu      sync.Mutex
	queries []countedQuery
}

type countedQuery struct {
	operation string
	sql       string
}

// NewQueryCounter returns an enabled counter that is not attached yet.
func NewQueryCounter() *QueryCounter {
	c := &QueryCounter{}
	c.enabled.Store(true)
	return c
}

// CaptureQueries attaches a counter to db for the rest of the test.
func CaptureQueries(tb testing.TB, db *bun.DB) *QueryCounter {
	tb.Helper()
	c := NewQueryCounter()
	db.AddQueryHook(c)
	tb.Cleanup(c.Stop)
	return c
}

// CaptureSettingValueSelects counts config.setting_values SELECTs (#2065).
func CaptureSettingValueSelects(db *bun.DB) func() int32 {
	c := NewQueryCounter()
	db.AddQueryHook(c)
	return func() int32 { return int32(len(c.Selects("config.setting_values"))) } //nolint:gosec // test-only count
}

func (c *QueryCounter) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	if !c.enabled.Load() {
		return ctx
	}
	c.mu.Lock()
	c.queries = append(c.queries, countedQuery{operation: event.Operation(), sql: event.Query})
	c.mu.Unlock()
	return ctx
}

func (*QueryCounter) AfterQuery(context.Context, *bun.QueryEvent) {}

// Start resumes recording after Stop.
func (c *QueryCounter) Start() { c.enabled.Store(true) }

// Stop pauses recording; already recorded statements stay readable.
func (c *QueryCounter) Stop() { c.enabled.Store(false) }

// Reset drops everything recorded so far.
func (c *QueryCounter) Reset() {
	c.mu.Lock()
	c.queries = nil
	c.mu.Unlock()
}

// Total is the number of statements recorded since the last Reset.
func (c *QueryCounter) Total() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.queries)
}

// Queries returns every recorded statement in issue order.
func (c *QueryCounter) Queries() []string {
	return c.Matching(func(string) bool { return true })
}

// Operation returns the statements of one bun operation ("SELECT", "INSERT", ...).
func (c *QueryCounter) Operation(operation string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for _, q := range c.queries {
		if q.operation == operation {
			out = append(out, q.sql)
		}
	}
	return out
}

// Selects returns the SELECT statements whose text contains table (case-insensitive).
func (c *QueryCounter) Selects(table string) []string {
	table = strings.ToLower(table)
	return c.Matching(func(sql string) bool {
		return strings.HasPrefix(strings.TrimSpace(sql), "select") && strings.Contains(sql, table)
	})
}

// Matching returns the statements whose lowercased text satisfies match.
func (c *QueryCounter) Matching(match func(sqlLower string) bool) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for _, q := range c.queries {
		if match(strings.ToLower(q.sql)) {
			out = append(out, q.sql)
		}
	}
	return out
}
