package data

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	iotModel "github.com/moto-nrw/project-phoenix/models/iot"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
	"github.com/stretchr/testify/assert"
)

type wireFeedback struct {
	available bool
	entry     feedbackModule.Entry
	err       error
}

func (f wireFeedback) Available(context.Context) (bool, error) { return f.available, nil }
func (f wireFeedback) Submit(context.Context, feedbackModule.CreateEntry) (feedbackModule.Entry, error) {
	return f.entry, f.err
}

type wireStudentReader struct{}

func (wireStudentReader) GetStudentByIDForUpdate(context.Context, int64) (*usersModel.Student, error) {
	return &usersModel.Student{Status: usersModel.StudentStatusActive}, nil
}

type wireObservation struct {
	status int
	code   string
}

func executeFeedbackWireRequest(t *testing.T, body string, feedback wireFeedback) (*httptest.ResponseRecorder, []wireObservation) {
	t.Helper()
	var observations []wireObservation
	resource := NewFeedbackResource(wireStudentReader{}, feedback, func(status int, code string) {
		observations = append(observations, wireObservation{status: status, code: code})
	}, nil)
	request := httptest.NewRequest(http.MethodPost, "/feedback", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	testutil.WithDeviceContext(&iotModel.Device{DeviceID: "wire-device"})(request)
	return testutil.ExecuteRequest(resource.Router(), request), observations
}

func TestIoTFeedbackWireContractsStayStable(t *testing.T) {
	t.Parallel()
	fixedTime := time.Date(2026, 8, 31, 10, 11, 12, 0, time.UTC)
	tests := []struct {
		name     string
		feedback wireFeedback
		request  string
		status   int
		code     string
		body     string
	}{
		{
			name: "success",
			feedback: wireFeedback{available: true, entry: feedbackModule.Entry{
				ID: 7, StudentID: 42, Value: feedbackModule.ValuePositive, Day: "2026-08-31", Time: "10:11:12", CreatedAt: fixedTime,
			}},
			request: `{"student_id":42,"value":"positive"}`,
			status:  http.StatusCreated,
			code:    "none",
			body:    `{"status":"success","data":{"created_at":"2026-08-31T10:11:12Z","day":"2026-08-31","id":7,"student_id":42,"time":"10:11:12","value":"positive"},"message":"Feedback submitted successfully"}` + "\n",
		},
		{
			name:     "disabled",
			feedback: wireFeedback{available: false},
			request:  `{"student_id":42,"value":"positive"}`,
			status:   http.StatusOK,
			code:     "feedback_disabled",
			body:     `{"status":"success","data":{"reason":"feedback_disabled","status":"skipped"},"message":"Feedback is disabled for this tenant"}` + "\n",
		},
		{
			name:     "mapped validation error",
			feedback: wireFeedback{available: true, err: &feedbackModule.InvalidEntryDataError{Err: errors.New("rejected")}},
			request:  `{"student_id":42,"value":"positive"}`,
			status:   http.StatusBadRequest,
			code:     "invalid_entry_data",
			body:     `{"status":"error","error":"invalid feedback entry data: rejected"}` + "\n",
		},
		{
			name:     "missing student ID",
			feedback: wireFeedback{available: true},
			request:  `{"value":"positive"}`,
			status:   http.StatusBadRequest,
			code:     "invalid_parameters",
			body:     `{"status":"error","error":"student_id is required and must be positive"}` + "\n",
		},
		{
			name:     "missing value",
			feedback: wireFeedback{available: true},
			request:  `{"student_id":42}`,
			status:   http.StatusBadRequest,
			code:     "invalid_parameters",
			body:     `{"status":"error","error":"value is required"}` + "\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder, observations := executeFeedbackWireRequest(t, test.request, test.feedback)
			assert.Equal(t, test.status, recorder.Code)
			assert.Equal(t, test.body, recorder.Body.String())
			assert.Equal(t, []wireObservation{{status: test.status, code: test.code}}, observations)
		})
	}
}
