package specialist

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	models2 "github.com/moto-nrw/project-phoenix/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestSpecialistLifecycle tests the complete lifecycle of a specialist
func TestSpecialistLifecycle(t *testing.T) {
	rs, mockSpecialistStore, _, mockUserStore := setupTestAPI()

	// 1. Setup test data
	now := time.Now()

	// Custom user for the specialist
	customUser := &models2.CustomUser{
		ID:         1,
		FirstName:  "John",
		SecondName: "Doe",
		TagID:      stringPtr("TEACHER001"),
		CreatedAt:  now,
		ModifiedAt: now,
	}

	// New specialist to create
	newSpecialist := &models2.PedagogicalSpecialist{
		Role: "Teacher",
		CustomUser: &models2.CustomUser{
			FirstName:  "John",
			SecondName: "Doe",
		},
	}

	// Specialist after creation
	createdSpecialist := &models2.PedagogicalSpecialist{
		ID:         1,
		Role:       "Teacher",
		UserID:     1,
		CustomUser: customUser,
		CreatedAt:  now,
		ModifiedAt: now,
		Groups:     []models2.Group{},
	}

	// Updated specialist
	updatedSpecialist := &models2.PedagogicalSpecialist{
		ID:         1,
		Role:       "Principal",
		UserID:     1,
		CustomUser: customUser,
		CreatedAt:  now,
		ModifiedAt: now,
		Groups:     []models2.Group{},
	}

	// Group for assignments
	group := &models2.Group{
		ID:   1,
		Name: "Test Group",
		Room: &models2.Room{
			ID:       1,
			RoomName: "Room 101",
		},
	}

	// 2. Set up expectations for creation
	mockSpecialistStore.On(
		"CreateSpecialist",
		mock.Anything,
		mock.MatchedBy(func(s *models2.PedagogicalSpecialist) bool {
			return s.Role == "Teacher"
		}),
		mock.MatchedBy(func(s *string) bool {
			return *s == "TEACHER001"
		}),
		mock.Anything,
	).Return(nil).Once()

	mockSpecialistStore.On("GetSpecialistByID", mock.Anything, int64(1)).Return(createdSpecialist, nil).Once()

	// 3. Set up expectations for retrieval
	mockSpecialistStore.On("GetSpecialistByID", mock.Anything, int64(1)).Return(createdSpecialist, nil).Once()

	// 4. Set up expectations for update
	mockSpecialistStore.On("UpdateSpecialist", mock.Anything, mock.MatchedBy(func(s *models2.PedagogicalSpecialist) bool {
		return s.ID == 1 && s.Role == "Principal"
	})).Return(nil).Once()

	mockSpecialistStore.On("GetSpecialistByID", mock.Anything, int64(1)).Return(updatedSpecialist, nil).Once()

	// 5. Set up expectations for group assignment
	mockSpecialistStore.On("AssignToGroup", mock.Anything, int64(1), int64(1)).Return(nil).Once()

	// 6. Set up expectations for listing assigned groups
	mockSpecialistStore.On("ListAssignedGroups", mock.Anything, int64(1)).Return([]models2.Group{*group}, nil).Once()

	// 7. Set up expectations for removing from group
	mockSpecialistStore.On("RemoveFromGroup", mock.Anything, int64(1), int64(1)).Return(nil).Once()

	// 8. Set up expectations for deletion
	mockSpecialistStore.On("DeleteSpecialist", mock.Anything, int64(1)).Return(nil).Once()

	// Use a standard context for testing
	standardContext := context.Background()

	// PHASE 1: Create Specialist
	t.Run("1. Create Specialist", func(t *testing.T) {
		// Create test request
		specialistReq := &SpecialistRequest{
			PedagogicalSpecialist: newSpecialist,
			TagID:                 "TEACHER001",
		}
		body, _ := json.Marshal(specialistReq)
		r := httptest.NewRequest("POST", "/", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")

		// Use the standard context
		r = r.WithContext(standardContext)

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.createSpecialist(w, r)

		// Check response
		assert.Equal(t, http.StatusCreated, w.Code)

		var responseSpecialist models2.PedagogicalSpecialist
		err := json.Unmarshal(w.Body.Bytes(), &responseSpecialist)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), responseSpecialist.ID)
		assert.Equal(t, "Teacher", responseSpecialist.Role)
		assert.Equal(t, "John", responseSpecialist.CustomUser.FirstName)
		assert.Equal(t, "Doe", responseSpecialist.CustomUser.SecondName)
	})

	// PHASE 2: Get Specialist
	t.Run("2. Get Specialist", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/1", nil)

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.getSpecialist(w, r)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		var responseSpecialist models2.PedagogicalSpecialist
		err := json.Unmarshal(w.Body.Bytes(), &responseSpecialist)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), responseSpecialist.ID)
		assert.Equal(t, "Teacher", responseSpecialist.Role)
	})

	// PHASE 3: Update Specialist
	t.Run("3. Update Specialist", func(t *testing.T) {
		// Create updated specialist request
		updateData := &models2.PedagogicalSpecialist{
			ID:         1,
			Role:       "Principal",
			UserID:     1,
			CustomUser: customUser,
		}

		specialistReq := &SpecialistRequest{
			PedagogicalSpecialist: updateData,
		}
		body, _ := json.Marshal(specialistReq)
		r := httptest.NewRequest("PUT", "/1", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.updateSpecialist(w, r)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		var responseSpecialist models2.PedagogicalSpecialist
		err := json.Unmarshal(w.Body.Bytes(), &responseSpecialist)
		assert.NoError(t, err)
		assert.Equal(t, "Principal", responseSpecialist.Role)
	})

	// PHASE 4: Assign to Group
	t.Run("4. Assign to Group", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/1/groups/1", nil)

		// Set URL parameters with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		rctx.URLParams.Add("groupId", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.assignToGroup(w, r)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.True(t, response["success"].(bool))
		assert.Equal(t, float64(1), response["specialist_id"])
		assert.Equal(t, float64(1), response["group_id"])
	})

	// PHASE 5: Get Assigned Groups
	t.Run("5. Get Assigned Groups", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/1/groups", nil)

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.getSpecialistGroups(w, r)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		var responseGroups []models2.Group
		err := json.Unmarshal(w.Body.Bytes(), &responseGroups)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(responseGroups))
		assert.Equal(t, "Test Group", responseGroups[0].Name)
	})

	// PHASE 6: Remove from Group
	t.Run("6. Remove from Group", func(t *testing.T) {
		r := httptest.NewRequest("DELETE", "/1/groups/1", nil)

		// Set URL parameters with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		rctx.URLParams.Add("groupId", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.removeFromGroup(w, r)

		// Check response
		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	// PHASE 7: Delete Specialist
	t.Run("7. Delete Specialist", func(t *testing.T) {
		r := httptest.NewRequest("DELETE", "/1", nil)

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.deleteSpecialist(w, r)

		// Check response
		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	// Verify all expectations were met
	mockSpecialistStore.AssertExpectations(t)
	mockUserStore.AssertExpectations(t)
}

// TestListAvailableSpecialists tests the listAvailableSpecialists handler
func TestListAvailableSpecialists(t *testing.T) {
	rs, mockSpecialistStore, _, _ := setupTestAPI()

	// Setup test data
	now := time.Now()

	specialists := []models2.PedagogicalSpecialist{
		{
			ID:     1,
			Role:   "Teacher",
			UserID: 1,
			CustomUser: &models2.CustomUser{
				ID:         1,
				FirstName:  "John",
				SecondName: "Doe",
			},
			CreatedAt:  now,
			ModifiedAt: now,
		},
		{
			ID:     2,
			Role:   "Assistant",
			UserID: 2,
			CustomUser: &models2.CustomUser{
				ID:         2,
				FirstName:  "Jane",
				SecondName: "Smith",
			},
			CreatedAt:  now,
			ModifiedAt: now,
		},
	}

	mockSpecialistStore.On("ListSpecialistsWithoutSupervision", mock.Anything).Return(specialists, nil)

	// Create test request
	r := httptest.NewRequest("GET", "/available", nil)
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.listAvailableSpecialists(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var responseSpecialists []models2.PedagogicalSpecialist
	err := json.Unmarshal(w.Body.Bytes(), &responseSpecialists)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(responseSpecialists))

	mockSpecialistStore.AssertExpectations(t)
}

// Helper function for string pointer
func stringPtr(s string) *string {
	return &s
}
