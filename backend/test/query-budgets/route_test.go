package querybudgets

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestMeasuredRouteMustMatchEvenWhenBothRoutesExist(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		stored, measured string
		valid            bool
	}{
		{"GET /api/students/", "GET /api/students/", true},
		{"GET /api/students/", "GET /api/pupils/", false},
		{"GET /api/students/", "", false},
		{"", "GET /api/students/", false},
		{"", "", true},
	} {
		if err := measuredRouteError(routeSet(tc.stored), routeSet(tc.measured)); (err == nil) != tc.valid {
			t.Errorf("stored %q measured %q: %v", tc.stored, tc.measured, err)
		}
	}
}

func routeSet(route string) []string {
	if route == "" {
		return nil
	}
	return []string{route}
}

// SQL slices must not be accepted: they lose whether HTTP was measured.
var _ func(testing.TB, string, Measurement) = AssertQueryBudget

type routeAssertionRecorder struct {
	testing.TB
	errors []string
}

func (*routeAssertionRecorder) Helper() {}
func (r *routeAssertionRecorder) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func TestHTTPBudgetRequiresRouteRegardlessOfScenarioPrefix(t *testing.T) {
	t.Parallel()
	getenv := func(string) string { return "" }
	counter := testpkg.NewQueryCounter()
	router := chi.NewRouter()
	router.Get("/parent/children/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	testpkg.MeasuredHTTPRequest(counter, router, httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/parent/children/", nil))
	measurement := counter.Measurement(counter.Queries())
	for _, scenario := range []string{"parent.new.http", "workforce.new.http", "operator.new.http", "communication18.new.http"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			missing := &routeAssertionRecorder{}
			assertQueryBudget(missing, scenario, queryBudget{max: 0}, measurement, getenv)
			if len(missing.errors) != 1 || !strings.Contains(missing.errors[0], "route:") {
				t.Fatalf("HTTP budget without route must fail: %v", missing.errors)
			}
			bound := &routeAssertionRecorder{}
			assertQueryBudget(bound, scenario, queryBudget{max: 0, route: measurement.Route}, measurement, getenv)
			if len(bound.errors) != 0 {
				t.Fatalf("bound HTTP budget failed: %v", bound.errors)
			}
		})
	}
	for name, handler := range map[string]http.Handler{
		"unmatched chi route":     router,
		"handler without routing": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			unmatched := testpkg.NewQueryCounter()
			testpkg.MeasuredHTTPRequest(unmatched, handler, httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/missing", nil))
			refused := &routeAssertionRecorder{}
			assertQueryBudget(refused, "parent.unmatched.http", queryBudget{max: 0}, unmatched.Measurement(nil), getenv)
			if len(refused.errors) != 1 || !strings.Contains(refused.errors[0], "no resolved route") {
				t.Fatalf("unmatched HTTP request must not become a non-HTTP budget: %v", refused.errors)
			}
		})
	}
	pure := &routeAssertionRecorder{}
	assertQueryBudget(pure, "parent.pure.use_case", queryBudget{max: 0}, testpkg.NewQueryCounter().Measurement(nil), getenv)
	if len(pure.errors) != 0 {
		t.Fatalf("non-HTTP measurement failed: %v", pure.errors)
	}
}
