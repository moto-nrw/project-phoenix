package test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"
)

func TestQueryBudgetMeasurementUsesFinalMountedRoute(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, path, pattern string
		sql                 bool
	}{
		{"table description without route", "/api/students/42/notes/7", "GET /api/students/{studentID}/notes/{noteID}", true},
		{"mount stub keeps golden trailing slash", "/api/students", "GET /api/students/", true},
		{"mounted list keeps golden trailing slash", "/api/students/", "GET /api/students/", true},
		{"zero SQL still has a request", "/api/students/42/notes/7", "GET /api/students/{studentID}/notes/{noteID}", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			counter := NewQueryCounter()
			root := chi.NewRouter()
			students := chi.NewRouter()
			handler := func(w http.ResponseWriter, req *http.Request) {
				if tc.sql {
					counter.BeforeQuery(req.Context(), &bun.QueryEvent{Query: "SELECT 1"})
				}
				w.WriteHeader(http.StatusNoContent)
			}
			students.Get("/", handler)
			students.Route("/{studentID}", func(r chi.Router) { r.Get("/notes/{noteID}", handler) })
			root.Route("/api", func(r chi.Router) { r.Mount("/students", students) })
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			MeasuredHTTPRequest(
				counter, root, httptest.NewRecorder(), req.WithContext(counter.Context(req.Context())))
			got := counter.Measurement(counter.Queries())
			if len(got.Route) != 1 || got.Route[0] != tc.pattern {
				t.Fatalf("route = %q, want %q", got.Route, tc.pattern)
			}
			if tc.sql && len(got.Statements) != 1 {
				t.Fatal("SQL measurement changed")
			}
			counter.Reset()
			if len(counter.Measurement(nil).Route) != 0 {
				t.Fatal("reset retained another request's route")
			}
		})
	}
}

func TestQueryBudgetMeasurementSortsAndDeduplicatesRequests(t *testing.T) {
	t.Parallel()
	counter := NewQueryCounter()
	router := chi.NewRouter()
	router.Get("/students/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	router.Get("/pupils/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	for _, path := range []string{"/students/3", "/pupils/", "/students/7"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		MeasuredHTTPRequest(counter, router, httptest.NewRecorder(), req)
	}
	got := counter.Measurement(counter.Queries())
	if len(got.Route) != 2 || got.Route[0] != "GET /pupils/" || got.Route[1] != "GET /students/{id}" {
		t.Fatalf("route set = %q", got.Route)
	}
	if len(got.Statements) != 0 {
		t.Fatal("recording routes changed the SQL bucket")
	}
}

func TestQueryBudgetMeasurementStopsAtTheRegisteredLeaf(t *testing.T) {
	t.Parallel()
	counter := NewQueryCounter()
	child := chi.NewRouter()
	child.Get("/students/{id}", func(w http.ResponseWriter, req *http.Request) {
		counter.BeforeQuery(req.Context(), &bun.QueryEvent{Query: "SELECT 1"})
		w.WriteHeader(http.StatusNoContent)
	})
	router := chi.NewRouter()
	router.Get("/students/{id}", child.ServeHTTP)
	MeasuredHTTPRequest(counter, router, httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/students/7", nil))
	got := counter.Measurement(counter.Queries())
	if len(got.Route) != 1 || got.Route[0] != "GET /students/{id}" {
		t.Fatalf("opaque delegation repeated its leaf: %q", got.Route)
	}
}

func TestMountedQueryBudgetRouterPreservesFixtureRequest(t *testing.T) {
	t.Parallel()
	counter := NewQueryCounter()
	resource := chi.NewRouter()
	resource.Get("/students/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodGet, "/students/42?limit=3", nil)
	response := httptest.NewRecorder()
	MeasuredHTTPRequest(counter, MountedQueryBudgetRouter("/api", resource), response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d", response.Code)
	}
	if request.URL.Path != "/students/42" || request.RequestURI != "/students/42?limit=3" || chi.RouteContext(request.Context()) != nil {
		t.Fatal("mounted fixture mutated the original request")
	}
	got := counter.Measurement(nil)
	if len(got.Route) != 1 || got.Route[0] != "GET /api/students/{id}" {
		t.Fatalf("mounted route = %q", got.Route)
	}
}
