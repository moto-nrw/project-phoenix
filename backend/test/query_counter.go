package test

import (
	"context"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/architecture/querybudget"
	"github.com/uptrace/bun"
)

// QueryCounter is the shared bun.QueryHook behind every query-budget test
// (#2940). It records each SQL statement issued through the hooked *bun.DB
// (statements inside tenant transactions included) while enabled, so a test
// can bound the total, one operation, or the statements touching one table.
//
// Attach it with CaptureQueries (registered with the pool's testQueryHooks
// dispatcher, removed again at cleanup) or hand NewQueryCounter to
// db.WithQueryHook for a private bun clone. An unscoped counter sees every
// statement on its pool while it is registered. Tests that share a package DB
// must use CaptureQueriesForContext and pass counter.Context(ctx) to the
// operation under test.
type QueryCounter struct {
	routeContexts      map[*chi.Context]bool
	enabled            atomic.Bool
	mu                 sync.Mutex
	queries            []countedQuery
	scope              uint64
	rowsAffected       int64
	statementsWithRows int
	writeRowsAffected  int64
}

type queryCounterContextKey struct{}

var queryCounterScopeID atomic.Uint64

type countedQuery struct {
	operation string
	sql       string
	deadline  time.Time
	issuedAt  time.Time
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
	registerTestQueryHook(tb, db, c, c.Stop)
	return c
}

// CaptureQueriesForContext attaches a counter that records only statements
// descended from counter.Context(ctx). Use it when parallel tests share db.
func CaptureQueriesForContext(tb testing.TB, db *bun.DB) *QueryCounter {
	tb.Helper()
	c := NewQueryCounter()
	c.scope = queryCounterScopeID.Add(1)
	registerTestQueryHook(tb, db, c, c.Stop)
	return c
}

// Context marks a request tree for a scoped counter. Values survive tenant
// transaction wrapping.
func (c *QueryCounter) Context(ctx context.Context) context.Context {
	if chi.RouteContext(ctx) == nil {
		ctx = context.WithValue(ctx, chi.RouteCtxKey, chi.NewRouteContext())
	}
	c.mu.Lock()
	if c.routeContexts == nil {
		c.routeContexts = map[*chi.Context]bool{}
	}
	c.routeContexts[chi.RouteContext(ctx)] = true
	c.mu.Unlock()
	if c.scope == 0 {
		return ctx
	}
	return context.WithValue(ctx, queryCounterContextKey{}, c.scope)
}

// CaptureSettingValueSelects counts config.setting_values SELECTs (#2065).
func CaptureSettingValueSelects(tb testing.TB, db *bun.DB) func() int32 {
	tb.Helper()
	c := CaptureQueries(tb, db)
	return func() int32 { return int32(len(c.Selects("config.setting_values"))) } //nolint:gosec // test-only count
}

func (c *QueryCounter) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	if !c.enabled.Load() {
		return ctx
	}
	if c.scope != 0 && ctx.Value(queryCounterContextKey{}) != c.scope {
		return ctx
	}
	c.mu.Lock()
	if route := chi.RouteContext(ctx); route != nil {
		if c.routeContexts == nil {
			c.routeContexts = map[*chi.Context]bool{}
		}
		c.routeContexts[route] = true
	}
	deadline, _ := ctx.Deadline()
	c.queries = append(c.queries, countedQuery{operation: event.Operation(), sql: event.Query, deadline: deadline, issuedAt: time.Now()})
	c.mu.Unlock()
	return ctx
}

func (c *QueryCounter) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	if !c.enabled.Load() || (c.scope != 0 && ctx.Value(queryCounterContextKey{}) != c.scope) || event.Result == nil {
		return
	}
	rows, err := event.Result.RowsAffected()
	if err != nil {
		return
	}
	c.mu.Lock()
	c.rowsAffected += rows
	c.statementsWithRows++
	if event.Err == nil {
		switch event.Operation() {
		case "INSERT", "UPDATE", "DELETE", "MERGE":
			c.writeRowsAffected += rows
		}
	}
	c.mu.Unlock()
}

// WriteRows returns driver-reported rows affected by successful DML statements.
// It excludes SELECTs, but includes writes later rolled back by the transaction.
func (c *QueryCounter) WriteRows() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writeRowsAffected
}

// Rows returns driver-reported rows and the number of statements with a known
// row count. SELECT rows are included; these are not distinct entity counts.
func (c *QueryCounter) Rows() (affected int64, statements int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.rowsAffected, c.statementsWithRows
}

// Start resumes recording after Stop.
func (c *QueryCounter) Start() { c.enabled.Store(true) }

// Stop pauses recording; already recorded statements stay readable.
func (c *QueryCounter) Stop() { c.enabled.Store(false) }

// Reset drops everything recorded so far.
func (c *QueryCounter) Reset() {
	c.mu.Lock()
	c.queries = nil
	c.routeContexts = nil
	c.rowsAffected = 0
	c.statementsWithRows = 0
	c.writeRowsAffected = 0
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

// StatementDeadline is a recorded statement with the moment it was issued and
// the deadline of the context it ran under; Deadline is zero when that
// context had none.
type StatementDeadline struct {
	SQL      string
	IssuedAt time.Time
	Deadline time.Time
}

// Deadlines returns every recorded statement with its context deadline, in
// issue order.
func (c *QueryCounter) Deadlines() []StatementDeadline {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]StatementDeadline, len(c.queries))
	for i, q := range c.queries {
		out[i] = StatementDeadline{SQL: q.sql, IssuedAt: q.issuedAt, Deadline: q.deadline}
	}
	return out
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

// Measurement snapshots the SQL bucket together with chi's final route pattern.
// Call only after ServeHTTP returns, when mounted child patterns are complete.
func (c *QueryCounter) Measurement(queries []string) querybudget.Measurement {
	c.mu.Lock()
	defer c.mu.Unlock()
	routes := map[string]bool{}
	result := querybudget.Measurement{Statements: queries}
	for route := range c.routeContexts {
		result.HTTPRequest = result.HTTPRequest || route.RouteMethod != ""
		if pattern := completedQueryBudgetRoute(route); pattern != "" {
			routes[route.RouteMethod+" "+pattern] = true
		}
	}
	for route := range routes {
		result.Route = append(result.Route, route)
	}
	slices.Sort(result.Route)
	return result
}

// QueryMeasurement is a request-aware SQL bucket for query-budget assertions.
type QueryMeasurement = querybudget.Measurement

// RoutePattern intentionally strips a trailing slash in chi 5.5. The router
// golden uses chi.Walk, which preserves a leaf "/". Restore only that actual
// matched leaf suffix, never a suffix inferred from the concrete request URL.
func completedQueryBudgetRoute(route *chi.Context) string {
	// Opaque dispatchers can route the same leaf twice in one context.
	// Unlike a mount, that repeated non-wildcard fragment adds no public path.
	// Keep real repeated mount prefixes ("/foo/*") and mount stubs followed
	// by their child "/", which chi.Walk preserves as a trailing slash.
	matched := *route
	matched.RoutePatterns = make([]string, 0, len(route.RoutePatterns))
	for _, fragment := range route.RoutePatterns {
		n := len(matched.RoutePatterns)
		if n > 0 && fragment == matched.RoutePatterns[n-1] && !strings.Contains(fragment, "*") {
			continue
		}
		matched.RoutePatterns = append(matched.RoutePatterns, fragment)
	}
	pattern := matched.RoutePattern()
	if pattern != "" && len(matched.RoutePatterns) > 0 && strings.HasSuffix(matched.RoutePatterns[len(matched.RoutePatterns)-1], "/") && !strings.HasSuffix(pattern, "/") {
		pattern += "/"
	}
	return pattern
}
