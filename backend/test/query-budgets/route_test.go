package querybudgets

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/architecture/querybudget"
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

func routeSet(route string) querybudget.Routes {
	if route == "" {
		return nil
	}
	return querybudget.Routes{route}
}
