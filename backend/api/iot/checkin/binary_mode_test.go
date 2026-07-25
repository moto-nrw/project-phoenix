package checkin

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type binaryModeActiveServiceStub struct {
	activeSvc.Service
	called   bool
	student  int64
	staff    int64
	device   int64
	skipAuth bool
}

func (s *binaryModeActiveServiceStub) ToggleStudentAttendance(
	_ context.Context,
	studentID, staffID, deviceID int64,
	skipAuthCheck bool,
) (*activeSvc.AttendanceResult, error) {
	s.called = true
	s.student = studentID
	s.staff = staffID
	s.device = deviceID
	s.skipAuth = skipAuthCheck
	return &activeSvc.AttendanceResult{
		Action:       "checked_in",
		AttendanceID: 99,
		StudentID:    studentID,
		Timestamp:    time.Now(),
	}, nil
}

func TestBinaryModeGreeting(t *testing.T) {
	tests := []struct {
		name   string
		action string
		first  string
		want   string
	}{
		{"check-in greets by first name", "checked_in", "Max", "Willkommen, Max!"},
		{"check-out farewells by first name", "checked_out", "Lena", "Tschüss, Lena!"},
		{"unknown action falls back to neutral message", "no_action", "Max", "Anwesenheit aktualisiert"},
		{"empty action falls back to neutral message", "", "Max", "Anwesenheit aktualisiert"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, binaryModeGreeting(tc.action, tc.first))
		})
	}
}

func TestProcessBinaryModeCheckinAllowsDeviceAttributionWithoutStaffContext(t *testing.T) {
	activeService := &binaryModeActiveServiceStub{}
	resource := &Resource{
		ActiveService: activeService,
		logger:        slog.New(slog.DiscardHandler),
	}
	student := &users.Student{
		Model: base.Model{ID: 42},
		Person: &users.Person{
			FirstName: "Max",
			LastName:  "Mustermann",
		},
	}
	kiosk := &iot.Device{Model: base.Model{ID: 7}}
	request := httptest.NewRequest(http.MethodPost, "/checkin", nil)
	response := httptest.NewRecorder()

	resource.processBinaryModeCheckin(response, request, student, kiosk, time.Now())

	require.Equal(t, http.StatusOK, response.Code)
	assert.True(t, activeService.called)
	assert.Equal(t, student.ID, activeService.student)
	assert.Zero(t, activeService.staff)
	assert.Equal(t, kiosk.ID, activeService.device)
	assert.True(t, activeService.skipAuth)
	assert.Contains(t, response.Body.String(), `"action":"checked_in"`)
}
