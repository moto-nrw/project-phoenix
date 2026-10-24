package careplanhttp_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanhttp "github.com/moto-nrw/project-phoenix/modules/careplan/http"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The handler tests drive the review over a fake capability and a recording
// surface: they pin the school transaction, the reference day and how each
// outcome reaches the operator surface. The root's binding to the real Care
// Plan is covered in api (TestOperatorBookingAuthorityReview*).

type fakeAuthority struct {
	impact *careplan.BookingAuthorityImpact
	err    error
	on     calendar.Date
	school int64
}

type schoolKey struct{}

func (f *fakeAuthority) PreviewBookingAuthorityImpact(ctx context.Context, on calendar.Date) (*careplan.BookingAuthorityImpact, error) {
	f.on = on
	f.school, _ = ctx.Value(schoolKey{}).(int64)
	return f.impact, f.err
}

// rendered is what the recording surface answered.
type rendered struct {
	status  int
	message string
	cause   error
	data    any
}

type recordedError struct{ rendered }

func (recordedError) Render(http.ResponseWriter, *http.Request) error { return nil }

func recordingSurface(out *rendered) careplanhttp.Surface {
	return careplanhttp.Surface{
		InvalidRequest: func(err error) render.Renderer {
			return recordedError{rendered{status: http.StatusBadRequest, message: err.Error()}}
		},
		Internal: func(message string, cause error) render.Renderer {
			return recordedError{rendered{status: http.StatusInternalServerError, message: message, cause: cause}}
		},
		RenderError: func(w http.ResponseWriter, _ *http.Request, renderer render.Renderer) {
			*out = renderer.(recordedError).rendered
			w.WriteHeader(out.status)
		},
		Respond: func(w http.ResponseWriter, _ *http.Request, status int, data any, message string) {
			*out = rendered{status: status, message: message, data: data}
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(data)
		},
	}
}

// withinSchool records the school and passes it on to the capability.
func withinSchool(ctx context.Context, schoolID int64, fn func(context.Context) error) error {
	return fn(context.WithValue(ctx, schoolKey{}, schoolID))
}

func serve(t *testing.T, authority careplanhttp.BookingAuthority, target string) (*httptest.ResponseRecorder, rendered) {
	t.Helper()
	var out rendered
	router := chi.NewRouter()
	router.Method(http.MethodGet, "/schools/{id}/settings/booking-authority-impact", careplanhttp.NewBookingAuthorityImpact(careplanhttp.BookingAuthorityImpactConfig{
		Authority:    authority,
		WithinSchool: withinSchool,
		Surface:      recordingSurface(&out),
	}))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, target, nil))
	return rr, out
}

func TestBookingAuthorityImpactPreviewsTodayInTheSchoolTransaction(t *testing.T) {
	t.Parallel()

	// synctest freezes this test's clock at 2000-01-01 UTC for this call.
	synctest.Test(t, func(t *testing.T) {
		impact := &careplan.BookingAuthorityImpact{
			ReferenceDate:    calendar.NewDate(2000, 1, 1),
			BlockingChildren: []careplan.BookingAuthorityImpactChild{{StudentID: "17", FirstName: "Ada"}},
		}
		authority := &fakeAuthority{impact: impact}

		rr, out := serve(t, authority, "/schools/4711/settings/booking-authority-impact")

		require.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, int64(4711), authority.school, "the preview runs in the transaction of the school in the path")
		assert.Equal(t, calendar.NewDate(2000, 1, 1), authority.on)
		assert.Same(t, impact, out.data)
		assert.Equal(t, "Booking authority impact retrieved successfully", out.message)
		assert.Contains(t, rr.Body.String(), `"blocking_children":[{"student_id":"17"`)
	})
}

func TestBookingAuthorityImpactRejectsAnInvalidSchoolID(t *testing.T) {
	t.Parallel()
	authority := &fakeAuthority{}

	rr, out := serve(t, authority, "/schools/abc/settings/booking-authority-impact")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "invalid school ID", out.message)
	assert.Zero(t, authority.school, "an invalid school never reaches Care Plan")
}

func TestBookingAuthorityImpactReportsAFailedPreview(t *testing.T) {
	t.Parallel()
	failure := errors.New("care plan unavailable")

	rr, out := serve(t, &fakeAuthority{err: failure}, "/schools/4711/settings/booking-authority-impact")

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, "Failed to review booking authority impact", out.message)
	assert.ErrorIs(t, out.cause, failure)
}

func TestNewBookingAuthorityImpactRequiresEveryDependency(t *testing.T) {
	t.Parallel()
	var out rendered
	complete := careplanhttp.BookingAuthorityImpactConfig{
		Authority:    &fakeAuthority{},
		WithinSchool: withinSchool,
		Surface:      recordingSurface(&out),
	}
	require.NotPanics(t, func() { careplanhttp.NewBookingAuthorityImpact(complete) })

	for name, drop := range map[string]func(*careplanhttp.BookingAuthorityImpactConfig){
		"authority":       func(c *careplanhttp.BookingAuthorityImpactConfig) { c.Authority = nil },
		"school runtime":  func(c *careplanhttp.BookingAuthorityImpactConfig) { c.WithinSchool = nil },
		"invalid request": func(c *careplanhttp.BookingAuthorityImpactConfig) { c.Surface.InvalidRequest = nil },
		"internal":        func(c *careplanhttp.BookingAuthorityImpactConfig) { c.Surface.Internal = nil },
		"render error":    func(c *careplanhttp.BookingAuthorityImpactConfig) { c.Surface.RenderError = nil },
		"respond":         func(c *careplanhttp.BookingAuthorityImpactConfig) { c.Surface.Respond = nil },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cfg := complete
			drop(&cfg)
			assert.Panics(t, func() { careplanhttp.NewBookingAuthorityImpact(cfg) })
		})
	}
}
