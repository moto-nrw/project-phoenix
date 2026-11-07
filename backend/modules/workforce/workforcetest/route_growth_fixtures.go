// Package workforcetest holds the owner-local fixtures used by Workforce tests.
package workforcetest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/uptrace/bun"
)

// RebookFixture exposes the route and persisted effects needed by the external
// growth tests. The HTTP package's internal test file binds its existing fixture.
type RebookFixture struct {
	DB                *bun.DB
	Router            http.Handler
	SubjectID, TypeID int64
	Absences          []int64
	Rebook            func(*testing.T, map[string]any) *httptest.ResponseRecorder
	Body              func(string, bool) map[string]any
	AugustBalance     func(*testing.T) int
	StoredTypes       func(*testing.T) []string
}

// The HTTP package binds these alongside its existing root composition seams.
var (
	OwnGrowthRouter     func(*testing.T) (*bun.DB, http.Handler)
	OwnGrowthToken      func(*testing.T, int64, int64, ...string) string
	RebookGrowthFixture func(*testing.T, int) RebookFixture
)
