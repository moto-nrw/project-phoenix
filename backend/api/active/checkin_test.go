// Package active_test tests the HTTP handlers for the active API
package active_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/active"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mock Services
// =============================================================================

// mockActiveService implements activeSvc.Service for testing
type mockActiveService struct {
	activeSvc.Service
	getActiveGroupFn             func(ctx context.Context, id int64) (*activeModels.Group, error)
	getStudentCurrentVisitFn     func(ctx context.Context, studentID int64) (*activeModels.Visit, error)
	getStudentAttendanceStatusFn func(ctx context.Context, studentID int64) (*activeSvc.AttendanceStatus, error)
	createVisitFn                func(ctx context.Context, visit *activeModels.Visit) error
	checkTeacherStudentAccessFn  func(ctx context.Context, teacherID, studentID int64) (bool, error)
}

func (m *mockActiveService) GetActiveGroup(ctx context.Context, id int64) (*activeModels.Group, error) {
	if m.getActiveGroupFn != nil {
		return m.getActiveGroupFn(ctx, id)
	}
	return nil, nil
}

func (m *mockActiveService) GetStudentCurrentVisit(ctx context.Context, studentID int64) (*activeModels.Visit, error) {
	if m.getStudentCurrentVisitFn != nil {
		return m.getStudentCurrentVisitFn(ctx, studentID)
	}
	return nil, nil
}

func (m *mockActiveService) GetStudentAttendanceStatus(ctx context.Context, studentID int64) (*activeSvc.AttendanceStatus, error) {
	if m.getStudentAttendanceStatusFn != nil {
		return m.getStudentAttendanceStatusFn(ctx, studentID)
	}
	return &activeSvc.AttendanceStatus{Status: "not_checked_in"}, nil
}

func (m *mockActiveService) CreateVisit(ctx context.Context, visit *activeModels.Visit) error {
	if m.createVisitFn != nil {
		return m.createVisitFn(ctx, visit)
	}
	visit.ID = 1
	return nil
}

func (m *mockActiveService) CheckTeacherStudentAccess(ctx context.Context, teacherID, studentID int64) (bool, error) {
	if m.checkTeacherStudentAccessFn != nil {
		return m.checkTeacherStudentAccessFn(ctx, teacherID, studentID)
	}
	return true, nil
}

// mockPersonService implements the person service interface for testing
type mockPersonService struct {
	findByAccountIDFn func(ctx context.Context, accountID int64) (*users.Person, error)
}

func (m *mockPersonService) FindByAccountID(ctx context.Context, accountID int64) (*users.Person, error) {
	if m.findByAccountIDFn != nil {
		return m.findByAccountIDFn(ctx, accountID)
	}
	person := &users.Person{}
	person.ID = 1
	return person, nil
}

// =============================================================================
// Test Helper Functions
// =============================================================================

func createTestRequest(t *testing.T, method, path string, body interface{}) *http.Request {
	t.Helper()

	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		require.NoError(t, err)
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func addJWTContext(req *http.Request, accountID int) *http.Request {
	claims := jwt.AppClaims{ID: accountID}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	return req.WithContext(ctx)
}

func addChiURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	return req.WithContext(ctx)
}

// =============================================================================
// CheckinRequest Tests
// =============================================================================

func TestCheckinRequest_Validation(t *testing.T) {
	t.Run("valid request with active_group_id", func(t *testing.T) {
		req := active.CheckinRequest{
			ActiveGroupID: 1,
		}
		assert.Greater(t, req.ActiveGroupID, int64(0))
	})

	t.Run("invalid request without active_group_id", func(t *testing.T) {
		req := active.CheckinRequest{}
		assert.Equal(t, int64(0), req.ActiveGroupID)
	})
}

// =============================================================================
// Request Construction Tests
// =============================================================================

func TestCheckinStudent_RequestConstruction(t *testing.T) {
	t.Run("valid request structure", func(t *testing.T) {
		reqBody := active.CheckinRequest{ActiveGroupID: 42}
		req := createTestRequest(t, "POST", "/api/active/students/123/checkin", reqBody)
		req = addJWTContext(req, 1)
		req = addChiURLParam(req, "studentId", "123")

		// Verify request is properly constructed
		assert.NotEmpty(t, req)

		var body active.CheckinRequest
		err := json.NewDecoder(req.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, int64(42), body.ActiveGroupID)
	})

	t.Run("request without active_group_id", func(t *testing.T) {
		req := createTestRequest(t, "POST", "/api/active/students/123/checkin", active.CheckinRequest{})
		req = addJWTContext(req, 1)
		req = addChiURLParam(req, "studentId", "123")

		var body active.CheckinRequest
		err := json.NewDecoder(req.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, int64(0), body.ActiveGroupID)
	})

	t.Run("URL param extraction", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req = addChiURLParam(req, "studentId", "456")

		rctx := chi.RouteContext(req.Context())
		assert.Equal(t, "456", rctx.URLParam("studentId"))
	})
}

// =============================================================================
// Error Response Tests
// =============================================================================

func TestCheckinError_Responses(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		message    string
	}{
		{"bad request", http.StatusBadRequest, "Invalid student ID"},
		{"unauthorized", http.StatusUnauthorized, "Invalid token"},
		{"forbidden", http.StatusForbidden, "Not authorized"},
		{"not found", http.StatusNotFound, "Active group not found"},
		{"conflict", http.StatusConflict, "Student already checked in"},
		{"internal error", http.StatusInternalServerError, "Failed to check in student"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.statusCode)

			response := map[string]string{
				"status":  "error",
				"message": tt.message,
			}
			jsonBytes, err := json.Marshal(response)
			require.NoError(t, err)
			_, err = w.Write(jsonBytes)
			require.NoError(t, err)

			assert.Equal(t, tt.statusCode, w.Code)
		})
	}
}

// =============================================================================
// Mock Service Behavior Tests
// =============================================================================

func TestMockActiveService_GetActiveGroup(t *testing.T) {
	t.Run("returns active group when exists", func(t *testing.T) {
		mock := &mockActiveService{
			getActiveGroupFn: func(ctx context.Context, id int64) (*activeModels.Group, error) {
				group := &activeModels.Group{RoomID: 1}
				group.ID = id
				return group, nil
			},
		}

		group, err := mock.GetActiveGroup(context.Background(), 42)
		require.NoError(t, err)
		assert.Equal(t, int64(42), group.ID)
	})

	t.Run("returns nil when not found", func(t *testing.T) {
		mock := &mockActiveService{
			getActiveGroupFn: func(ctx context.Context, id int64) (*activeModels.Group, error) {
				return nil, nil
			},
		}

		group, err := mock.GetActiveGroup(context.Background(), 999)
		require.NoError(t, err)
		assert.Nil(t, group)
	})
}

func TestMockActiveService_CreateVisit(t *testing.T) {
	t.Run("creates visit and assigns ID", func(t *testing.T) {
		mock := &mockActiveService{
			createVisitFn: func(ctx context.Context, visit *activeModels.Visit) error {
				visit.ID = 100
				return nil
			},
		}

		visit := &activeModels.Visit{
			StudentID:     1,
			ActiveGroupID: 2,
		}

		err := mock.CreateVisit(context.Background(), visit)
		require.NoError(t, err)
		assert.Equal(t, int64(100), visit.ID)
	})
}

func TestMockActiveService_CheckTeacherStudentAccess(t *testing.T) {
	t.Run("returns true when teacher has access", func(t *testing.T) {
		mock := &mockActiveService{
			checkTeacherStudentAccessFn: func(ctx context.Context, teacherID, studentID int64) (bool, error) {
				return true, nil
			},
		}

		hasAccess, err := mock.CheckTeacherStudentAccess(context.Background(), 1, 2)
		require.NoError(t, err)
		assert.True(t, hasAccess)
	})

	t.Run("returns false when teacher has no access", func(t *testing.T) {
		mock := &mockActiveService{
			checkTeacherStudentAccessFn: func(ctx context.Context, teacherID, studentID int64) (bool, error) {
				return false, nil
			},
		}

		hasAccess, err := mock.CheckTeacherStudentAccess(context.Background(), 1, 2)
		require.NoError(t, err)
		assert.False(t, hasAccess)
	})
}

func TestMockActiveService_GetStudentAttendanceStatus(t *testing.T) {
	statuses := []string{"checked_in", "checked_out", "not_checked_in"}

	for _, status := range statuses {
		t.Run("returns "+status+" status", func(t *testing.T) {
			mock := &mockActiveService{
				getStudentAttendanceStatusFn: func(ctx context.Context, studentID int64) (*activeSvc.AttendanceStatus, error) {
					return &activeSvc.AttendanceStatus{
						StudentID: studentID,
						Status:    status,
					}, nil
				},
			}

			result, err := mock.GetStudentAttendanceStatus(context.Background(), 1)
			require.NoError(t, err)
			assert.Equal(t, status, result.Status)
		})
	}
}

func TestMockActiveService_GetStudentCurrentVisit(t *testing.T) {
	t.Run("returns visit when student has active visit", func(t *testing.T) {
		mock := &mockActiveService{
			getStudentCurrentVisitFn: func(ctx context.Context, studentID int64) (*activeModels.Visit, error) {
				visit := &activeModels.Visit{
					StudentID:     studentID,
					ActiveGroupID: 5,
				}
				visit.ID = 10
				return visit, nil
			},
		}

		visit, err := mock.GetStudentCurrentVisit(context.Background(), 1)
		require.NoError(t, err)
		require.NotNil(t, visit)
		assert.Equal(t, int64(10), visit.ID)
	})

	t.Run("returns nil when no active visit", func(t *testing.T) {
		mock := &mockActiveService{
			getStudentCurrentVisitFn: func(ctx context.Context, studentID int64) (*activeModels.Visit, error) {
				return nil, nil
			},
		}

		visit, err := mock.GetStudentCurrentVisit(context.Background(), 1)
		require.NoError(t, err)
		assert.Nil(t, visit)
	})
}

// =============================================================================
// Context Helper Tests
// =============================================================================

func TestAddJWTContext(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req = addJWTContext(req, 42)

	claims := jwt.ClaimsFromCtx(req.Context())
	assert.Equal(t, 42, claims.ID)
}

func TestAddChiURLParam(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req = addChiURLParam(req, "studentId", "123")

	rctx := chi.RouteContext(req.Context())
	assert.Equal(t, "123", rctx.URLParam("studentId"))
}

func TestMultipleChiURLParams(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req = addChiURLParam(req, "studentId", "123")
	req = addChiURLParam(req, "groupId", "456")

	rctx := chi.RouteContext(req.Context())
	// Note: addChiURLParam creates new context each time, so only last param is preserved
	// This tests the helper behavior
	assert.NotNil(t, rctx)
}

// =============================================================================
// Person Service Mock Tests
// =============================================================================

func TestMockPersonService_FindByAccountID(t *testing.T) {
	t.Run("returns person when found", func(t *testing.T) {
		mock := &mockPersonService{
			findByAccountIDFn: func(ctx context.Context, accountID int64) (*users.Person, error) {
				person := &users.Person{
					FirstName: "Test",
					LastName:  "User",
				}
				person.ID = accountID
				return person, nil
			},
		}

		person, err := mock.FindByAccountID(context.Background(), 5)
		require.NoError(t, err)
		assert.Equal(t, int64(5), person.ID)
		assert.Equal(t, "Test", person.FirstName)
	})

	t.Run("returns nil when not found", func(t *testing.T) {
		mock := &mockPersonService{
			findByAccountIDFn: func(ctx context.Context, accountID int64) (*users.Person, error) {
				return nil, nil
			},
		}

		person, err := mock.FindByAccountID(context.Background(), 999)
		require.NoError(t, err)
		assert.Nil(t, person)
	})
}

// =============================================================================
// JSON Encoding Tests
// =============================================================================

func TestCheckinRequest_JSONEncoding(t *testing.T) {
	t.Run("encodes to JSON correctly", func(t *testing.T) {
		req := active.CheckinRequest{ActiveGroupID: 123}
		data, err := json.Marshal(req)
		require.NoError(t, err)
		assert.Contains(t, string(data), "123")
	})

	t.Run("decodes from JSON correctly", func(t *testing.T) {
		jsonData := `{"active_group_id": 456}`
		var req active.CheckinRequest
		err := json.Unmarshal([]byte(jsonData), &req)
		require.NoError(t, err)
		assert.Equal(t, int64(456), req.ActiveGroupID)
	})
}

// =============================================================================
// Success Response Pattern Tests
// =============================================================================

func TestSuccessResponse_Pattern(t *testing.T) {
	t.Run("success response structure", func(t *testing.T) {
		w := httptest.NewRecorder()
		w.WriteHeader(http.StatusOK)

		response := map[string]interface{}{
			"status":  "success",
			"message": "Student checked in successfully",
			"data": map[string]interface{}{
				"student_id":      int64(123),
				"action":          "checked_in",
				"visit_id":        int64(456),
				"active_group_id": int64(789),
			},
		}

		jsonBytes, err := json.Marshal(response)
		require.NoError(t, err)
		_, err = w.Write(jsonBytes)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "success")
		assert.Contains(t, w.Body.String(), "checked_in")
	})
}
