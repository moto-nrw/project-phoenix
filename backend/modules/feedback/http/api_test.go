package feedback_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
	feedbackHTTP "github.com/moto-nrw/project-phoenix/modules/feedback/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type engine struct {
	available    bool
	availableErr error
	entries      []feedbackModule.Entry
	err          error
}

func (e engine) Available(context.Context) (bool, error) { return e.available, e.availableErr }
func (e engine) Submit(_ context.Context, input feedbackModule.CreateEntry) (feedbackModule.Entry, error) {
	return feedbackModule.Entry{ID: 7, Value: input.Value, Day: input.Day, Time: input.Time, StudentID: input.StudentID}, e.err
}
func (e engine) SubmitBatch(context.Context, []feedbackModule.CreateEntry) ([]feedbackModule.Entry, error) {
	return e.entries, e.err
}
func (e engine) LookupEntry(context.Context, int64) (feedbackModule.Entry, error) {
	if e.err != nil {
		return feedbackModule.Entry{}, e.err
	}
	return e.entries[0], nil
}
func (e engine) EraseEntry(context.Context, int64) error { return e.err }
func (e engine) FindEntries(context.Context, feedbackModule.Filter) ([]feedbackModule.Entry, error) {
	return e.entries, e.err
}
func (e engine) DeleteExpired(context.Context) (int, error)          { return 0, e.err }
func (e engine) CountForStudent(context.Context, int64) (int, error) { return 0, e.err }
func (e engine) ObserveRejection(string, time.Duration, error)       {}

type response struct {
	Status  string `json:"status"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
}

func (response) Render(http.ResponseWriter, *http.Request) error { return nil }

type errorResponse struct {
	StatusCode int    `json:"-"`
	Status     string `json:"status"`
	Error      string `json:"error"`
}

func (e *errorResponse) Render(http.ResponseWriter, *http.Request) error { return nil }

func testResource(e engine) *feedbackHTTP.Resource {
	return testResourceWithObserver(e, func(int, string) {})
}

func testResourceWithObserver(e engine, observe func(int, string)) *feedbackHTTP.Resource {
	return feedbackHTTP.NewResource(feedbackModule.NewModule(e), feedbackHTTP.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, feedbackHTTP.Middleware)) {
			register(router, func(next http.Handler) http.Handler { return next })
		},
		Permission: func(string) feedbackHTTP.Middleware {
			return func(next http.Handler) http.Handler { return next }
		},
		Success: func(w http.ResponseWriter, r *http.Request, status int, data any, message string) {
			render.Status(r, status)
			require.NoError(testingT(r.Context()), render.Render(w, r, &response{Status: "success", Data: data, Message: message}))
		},
		Failure: func(w http.ResponseWriter, r *http.Request, failure feedbackHTTP.Failure) {
			render.Status(r, failure.Status)
			require.NoError(testingT(r.Context()), render.Render(w, r, &errorResponse{StatusCode: failure.Status, Status: failure.Classification, Error: failure.Err.Error()}))
		},
		ObserveResponse: observe,
	})
}

type testKey struct{}

func testingT(ctx context.Context) *testing.T { return ctx.Value(testKey{}).(*testing.T) }

func execute(t *testing.T, resource *feedbackHTTP.Resource, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var encoded bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&encoded).Encode(body))
	}
	request := httptest.NewRequest(method, target, &encoded).WithContext(context.WithValue(context.Background(), testKey{}, t))
	recorder := httptest.NewRecorder()
	resource.Router().ServeHTTP(recorder, request)
	return recorder
}

func TestStaffWireContractsStayStable(t *testing.T) {
	t.Parallel()
	entry := feedbackModule.Entry{ID: 7, Value: feedbackModule.ValuePositive, Day: "2026-08-31", Time: "12:34:56", StudentID: 42}
	entryJSON := `{"id":7,"value":"positive","day":"2026-08-31","time":"12:34:56","student_id":42,"is_mensa_feedback":false,"created_at":"0001-01-01T00:00:00Z","updated_at":"0001-01-01T00:00:00Z"}`
	tests := []struct {
		name, method, target, want string
		body                       any
		status                     int
	}{
		{"list", http.MethodGet, "/", `{"status":"success","data":[` + entryJSON + `],"message":"Feedback entries retrieved successfully"}` + "\n", nil, http.StatusOK},
		{"get", http.MethodGet, "/7", `{"status":"success","data":` + entryJSON + `,"message":"Feedback entry retrieved successfully"}` + "\n", nil, http.StatusOK},
		{"student", http.MethodGet, "/student/42", `{"status":"success","data":[` + entryJSON + `],"message":"Student feedback entries retrieved successfully"}` + "\n", nil, http.StatusOK},
		{"date", http.MethodGet, "/date/2026-08-31", `{"status":"success","data":[` + entryJSON + `],"message":"Date feedback entries retrieved successfully"}` + "\n", nil, http.StatusOK},
		{"mensa", http.MethodGet, "/mensa", `{"status":"success","data":[` + entryJSON + `],"message":"Mensa feedback entries retrieved successfully"}` + "\n", nil, http.StatusOK},
		{"date range", http.MethodGet, "/date-range?start_date=2026-08-01&end_date=2026-08-31", `{"status":"success","data":[` + entryJSON + `],"message":"Date range feedback entries retrieved successfully"}` + "\n", nil, http.StatusOK},
		{"create", http.MethodPost, "/", `{"status":"success","data":` + entryJSON + `,"message":"Feedback entry created successfully"}` + "\n", map[string]any{"value": "positive", "day": "2026-08-31", "time": "12:34:56", "student_id": 42}, http.StatusCreated},
		{"batch", http.MethodPost, "/batch", `{"status":"success","data":{"count":1},"message":"Feedback entries created successfully"}` + "\n", map[string]any{"entries": []map[string]any{{"value": "positive", "day": "2026-08-31", "time": "12:34:56", "student_id": 42}}}, http.StatusCreated},
		{"delete", http.MethodDelete, "/7", `{"status":"success","message":"Feedback entry deleted successfully"}` + "\n", nil, http.StatusOK},
	}
	resource := testResource(engine{available: true, entries: []feedbackModule.Entry{entry}})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := execute(t, resource, test.method, test.target, test.body)
			assert.Equal(t, test.status, recorder.Code)
			assert.Equal(t, test.want, recorder.Body.String())
		})
	}
}

func TestStaffErrorWireClassificationsStayStable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		code int
		body string
	}{
		{"not found", feedbackModule.ErrEntryNotFound, 404, `{"status":"Resource Not Found","error":"feedback entry not found"}`},
		{"invalid", feedbackModule.ErrInvalidEntryData, 400, `{"status":"Invalid Feedback Data","error":"invalid feedback entry data"}`},
		{"range", feedbackModule.ErrInvalidDateRange, 400, `{"status":"Invalid Date Range","error":"invalid date range"}`},
		{"student", feedbackModule.ErrStudentNotFound, 404, `{"status":"Student Not Found","error":"student not found"}`},
		{"internal", errors.New("boom"), 500, `{"status":"Internal Server Error","error":"boom"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			recorder := execute(t, testResource(engine{err: test.err}), http.MethodGet, "/7", nil)
			assert.Equal(t, test.code, recorder.Code)
			assert.Equal(t, test.body+"\n", recorder.Body.String())
		})
	}
}

func TestStudentReadDoesNotSwallowSettingsFailure(t *testing.T) {
	t.Parallel()
	recorder := execute(t, testResource(engine{availableErr: errors.New("settings down")}), http.MethodGet, "/student/42", nil)
	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "settings down")
}

func TestStaffResponseObserverUsesActualStatusAndStableCode(t *testing.T) {
	t.Parallel()
	type observation struct {
		status int
		code   string
	}
	var observations []observation
	resource := testResourceWithObserver(engine{err: feedbackModule.ErrEntryNotFound}, func(status int, code string) {
		observations = append(observations, observation{status: status, code: code})
	})

	execute(t, resource, http.MethodGet, "/invalid", nil)
	execute(t, resource, http.MethodGet, "/7", nil)

	assert.Equal(t, []observation{
		{status: http.StatusBadRequest, code: "invalid_parameters"},
		{status: http.StatusNotFound, code: "entry_not_found"},
	}, observations)
}
