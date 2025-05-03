package group

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

// TestGroupLifecycle tests the complete lifecycle of a group
func TestGroupLifecycle(t *testing.T) {
	rs, mockGroupStore, _ := setupTestAPI()

	// 1. Setup test data
	now := time.Now()

	// Room for the group
	room := &models2.Room{
		ID:         101,
		RoomName:   "Classroom 101",
		Building:   "Main Building",
		Floor:      1,
		Capacity:   30,
		CreatedAt:  now,
		ModifiedAt: now,
	}

	// Supervisors for the group
	supervisor1 := &models2.PedagogicalSpecialist{
		ID:   1,
		Role: "Teacher",
		CustomUser: &models2.CustomUser{
			ID:         1,
			FirstName:  "John",
			SecondName: "Smith",
		},
	}

	supervisor2 := &models2.PedagogicalSpecialist{
		ID:   2,
		Role: "Teacher",
		CustomUser: &models2.CustomUser{
			ID:         2,
			FirstName:  "Jane",
			SecondName: "Doe",
		},
	}

	// 2. Define test group
	newGroup := &models2.Group{
		Name:   "New Test Group",
		RoomID: &room.ID,
	}

	// Group after creation
	createdGroup := &models2.Group{
		ID:         1,
		Name:       "New Test Group",
		RoomID:     &room.ID,
		Room:       room,
		CreatedAt:  now,
		ModifiedAt: now,
		Supervisors: []*models2.PedagogicalSpecialist{
			supervisor1,
		},
		Students: []models2.Student{},
	}

	// Group after update
	updatedGroup := &models2.Group{
		ID:         1,
		Name:       "Updated Group Name", // Changed
		RoomID:     &room.ID,
		Room:       room,
		CreatedAt:  now,
		ModifiedAt: now.Add(time.Minute),
		Supervisors: []*models2.PedagogicalSpecialist{
			supervisor1,
			supervisor2, // Added
		},
		Students: []models2.Student{},
	}

	// Representative specialist ID
	representativeID := int64(1) // Using supervisor1's ID

	// Group with representative
	groupWithRepresentative := &models2.Group{
		ID:               1,
		Name:             "Updated Group Name",
		RoomID:           &room.ID,
		Room:             room,
		RepresentativeID: &representativeID,
		CreatedAt:        now,
		ModifiedAt:       now.Add(2 * time.Minute),
		Supervisors: []*models2.PedagogicalSpecialist{
			supervisor1,
			supervisor2,
		},
		Students: []models2.Student{},
	}

	// Combined group data
	newCombinedGroup := &models2.CombinedGroup{
		Name:         "Combined Group A+B",
		IsActive:     true,
		AccessPolicy: "all",
	}

	// Combined group after creation
	createdCombinedGroup := &models2.CombinedGroup{
		ID:           1,
		Name:         "Combined Group A+B",
		IsActive:     true,
		AccessPolicy: "all",
		Groups: []*models2.Group{
			{ID: 1, Name: "Updated Group Name"},
		},
		AccessSpecialists: []*models2.PedagogicalSpecialist{
			supervisor1,
		},
		CreatedAt: now,
	}

	// 3. Set up expectations for creation
	mockGroupStore.On("CreateGroup", mock.Anything, newGroup, []int64{1}).Return(nil)
	mockGroupStore.On("GetGroupByID", mock.Anything, int64(1)).Return(createdGroup, nil).Times(1)

	// 4. Set up expectations for update
	mockGroupStore.On("GetGroupByID", mock.Anything, int64(1)).Return(createdGroup, nil).Times(1)
	mockGroupStore.On("UpdateGroup", mock.Anything, mock.MatchedBy(func(g *models2.Group) bool {
		return g.ID == 1 && g.Name == "Updated Group Name"
	})).Return(nil)
	mockGroupStore.On("GetGroupByID", mock.Anything, int64(1)).Return(updatedGroup, nil).Times(1)

	// 5. Set up expectations for supervisor update
	mockGroupStore.On("GetGroupByID", mock.Anything, int64(1)).Return(updatedGroup, nil).Times(1)
	mockGroupStore.On("UpdateGroupSupervisors", mock.Anything, int64(1), []int64{1, 2}).Return(nil)
	mockGroupStore.On("GetGroupByID", mock.Anything, int64(1)).Return(updatedGroup, nil).Times(1)

	// 6. Set up expectations for setting representative
	mockGroupStore.On("GetGroupByID", mock.Anything, int64(1)).Return(updatedGroup, nil).Times(1)
	mockGroupStore.On("UpdateGroup", mock.Anything, mock.MatchedBy(func(g *models2.Group) bool {
		return g.ID == 1 && g.RepresentativeID != nil && *g.RepresentativeID == 1
	})).Return(nil)
	mockGroupStore.On("GetGroupByID", mock.Anything, int64(1)).Return(groupWithRepresentative, nil).Times(1)

	// 7. Set up expectations for combined group creation
	mockGroupStore.On("CreateCombinedGroup", mock.Anything, mock.MatchedBy(func(cg *models2.CombinedGroup) bool {
		return cg.Name == "Combined Group A+B" && cg.AccessPolicy == "all"
	}), []int64{1}, []int64{1}).Return(nil)
	mockGroupStore.On("GetCombinedGroupByID", mock.Anything, int64(1)).Return(createdCombinedGroup, nil)

	// 8. Set up expectations for group deletion
	mockGroupStore.On("DeleteGroup", mock.Anything, int64(1)).Return(nil)

	// Use a standard context for testing
	standardContext := context.Background()

	// PHASE 1: Create Group
	t.Run("1. Create Group", func(t *testing.T) {
		// Create test request
		groupReq := &GroupRequest{
			Group:         newGroup,
			SupervisorIDs: []int64{1},
		}
		body, _ := json.Marshal(groupReq)
		r := httptest.NewRequest("POST", "/", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")

		// Use the standard context
		r = r.WithContext(standardContext)

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.createGroup(w, r)

		// Check response
		assert.Equal(t, http.StatusCreated, w.Code)

		var responseGroup models2.Group
		err := json.Unmarshal(w.Body.Bytes(), &responseGroup)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), responseGroup.ID)
		assert.Equal(t, "New Test Group", responseGroup.Name)
		assert.Equal(t, int64(101), *responseGroup.RoomID)
		assert.Equal(t, 1, len(responseGroup.Supervisors))
	})

	// PHASE 2: Update Group
	t.Run("2. Update Group", func(t *testing.T) {
		// Create updated group request
		updateData := &models2.Group{
			ID:     1,
			Name:   "Updated Group Name",
			RoomID: &room.ID,
		}

		groupReq := &GroupRequest{Group: updateData}
		body, _ := json.Marshal(groupReq)
		r := httptest.NewRequest("PUT", "/1", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.updateGroup(w, r)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		var responseGroup models2.Group
		err := json.Unmarshal(w.Body.Bytes(), &responseGroup)
		assert.NoError(t, err)
		assert.Equal(t, "Updated Group Name", responseGroup.Name)
	})

	// PHASE 3: Update Group Supervisors
	t.Run("3. Update Group Supervisors", func(t *testing.T) {
		// Create supervisor update request
		supervisorReq := &SupervisorRequest{
			SupervisorIDs: []int64{1, 2},
		}

		body, _ := json.Marshal(supervisorReq)
		r := httptest.NewRequest("POST", "/1/supervisors", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.updateGroupSupervisors(w, r)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		var responseGroup models2.Group
		err := json.Unmarshal(w.Body.Bytes(), &responseGroup)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(responseGroup.Supervisors))
	})

	// PHASE 4: Set Group Representative
	t.Run("4. Set Group Representative", func(t *testing.T) {
		// Create representative request
		repReq := &RepresentativeRequest{
			SpecialistID: 1, // Using supervisor1's ID
		}

		body, _ := json.Marshal(repReq)
		r := httptest.NewRequest("POST", "/1/representative", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.setGroupRepresentative(w, r)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		var responseGroup models2.Group
		err := json.Unmarshal(w.Body.Bytes(), &responseGroup)
		assert.NoError(t, err)
		assert.NotNil(t, responseGroup.RepresentativeID)
		assert.Equal(t, int64(1), *responseGroup.RepresentativeID)
	})

	// PHASE 5: Create Combined Group
	t.Run("5. Create Combined Group", func(t *testing.T) {
		// Create combined group request
		combinedGroupReq := &CombinedGroupRequest{
			CombinedGroup: newCombinedGroup,
			GroupIDs:      []int64{1},
			SpecialistIDs: []int64{1},
		}

		body, _ := json.Marshal(combinedGroupReq)
		r := httptest.NewRequest("POST", "/combined", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.createCombinedGroup(w, r)

		// Check response
		assert.Equal(t, http.StatusCreated, w.Code)

		var responseCombinedGroup models2.CombinedGroup
		err := json.Unmarshal(w.Body.Bytes(), &responseCombinedGroup)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), responseCombinedGroup.ID)
		assert.Equal(t, "Combined Group A+B", responseCombinedGroup.Name)
		assert.Equal(t, 1, len(responseCombinedGroup.Groups))
		assert.Equal(t, 1, len(responseCombinedGroup.AccessSpecialists))
	})

	// PHASE 6: Delete Group
	t.Run("6. Delete Group", func(t *testing.T) {
		r := httptest.NewRequest("DELETE", "/1", nil)

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.deleteGroup(w, r)

		// Check response
		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	// Verify all expectations were met
	mockGroupStore.AssertExpectations(t)
}

// TestCombinedGroupLifecycle tests the complete lifecycle of a combined group
func TestCombinedGroupLifecycle(t *testing.T) {
	rs, mockGroupStore, _ := setupTestAPI()

	// 1. Setup test data
	now := time.Now()
	validUntil := now.Add(24 * time.Hour)

	// Groups for the combined group
	group1 := &models2.Group{
		ID:   1,
		Name: "Group A",
	}

	group2 := &models2.Group{
		ID:   2,
		Name: "Group B",
	}

	// Specialists for access control
	specialist1 := &models2.PedagogicalSpecialist{
		ID:   1,
		Role: "Teacher",
		CustomUser: &models2.CustomUser{
			ID:         1,
			FirstName:  "John",
			SecondName: "Smith",
		},
	}

	specialist2 := &models2.PedagogicalSpecialist{
		ID:   2,
		Role: "Teacher",
		CustomUser: &models2.CustomUser{
			ID:         2,
			FirstName:  "Jane",
			SecondName: "Doe",
		},
	}

	// 2. Define test combined group
	newCombinedGroup := &models2.CombinedGroup{
		Name:         "New Combined Group",
		IsActive:     true,
		AccessPolicy: "all",
		ValidUntil:   &validUntil,
	}

	// Combined group after creation
	createdCombinedGroup := &models2.CombinedGroup{
		ID:           1,
		Name:         "New Combined Group",
		IsActive:     true,
		AccessPolicy: "all",
		ValidUntil:   &validUntil,
		Groups: []*models2.Group{
			group1,
		},
		AccessSpecialists: []*models2.PedagogicalSpecialist{
			specialist1,
		},
		CreatedAt: now,
	}

	// Combined group after adding another group
	updatedCombinedGroup := &models2.CombinedGroup{
		ID:           1,
		Name:         "New Combined Group",
		IsActive:     true,
		AccessPolicy: "all",
		ValidUntil:   &validUntil,
		Groups: []*models2.Group{
			group1,
			group2, // Added group
		},
		AccessSpecialists: []*models2.PedagogicalSpecialist{
			specialist1,
		},
		CreatedAt: now,
	}

	// Combined group after updating specialists
	finalCombinedGroup := &models2.CombinedGroup{
		ID:           1,
		Name:         "New Combined Group",
		IsActive:     true,
		AccessPolicy: "all",
		ValidUntil:   &validUntil,
		Groups: []*models2.Group{
			group1,
			group2,
		},
		AccessSpecialists: []*models2.PedagogicalSpecialist{
			specialist1,
			specialist2, // Added specialist
		},
		CreatedAt: now,
	}

	// 3. Set up expectations for creation
	mockGroupStore.On("CreateCombinedGroup", mock.Anything, mock.MatchedBy(func(cg *models2.CombinedGroup) bool {
		return cg.Name == "New Combined Group" && cg.AccessPolicy == "all"
	}), []int64{1}, []int64{1}).Return(nil)
	mockGroupStore.On("GetCombinedGroupByID", mock.Anything, int64(1)).Return(createdCombinedGroup, nil).Times(1)

	// 4. Set up expectations for adding a group
	mockGroupStore.On("GetCombinedGroupByID", mock.Anything, int64(1)).Return(createdCombinedGroup, nil).Times(1)
	mockGroupStore.On("CreateCombinedGroup", mock.Anything, mock.MatchedBy(func(cg *models2.CombinedGroup) bool {
		return cg.ID == 1
	}), []int64{1, 2}, []int64{1}).Return(nil)
	mockGroupStore.On("GetCombinedGroupByID", mock.Anything, int64(1)).Return(updatedCombinedGroup, nil).Times(1)

	// 5. Set up expectations for updating specialists
	mockGroupStore.On("GetCombinedGroupByID", mock.Anything, int64(1)).Return(updatedCombinedGroup, nil).Times(1)
	mockGroupStore.On("CreateCombinedGroup", mock.Anything, mock.MatchedBy(func(cg *models2.CombinedGroup) bool {
		return cg.ID == 1
	}), []int64{1, 2}, []int64{1, 2}).Return(nil)
	mockGroupStore.On("GetCombinedGroupByID", mock.Anything, int64(1)).Return(finalCombinedGroup, nil).Times(1)

	// 6. Set up expectations for deactivation (deletion)
	mockGroupStore.On("GetCombinedGroupByID", mock.Anything, int64(1)).Return(finalCombinedGroup, nil).Times(1)
	mockGroupStore.On("CreateCombinedGroup", mock.Anything, mock.MatchedBy(func(cg *models2.CombinedGroup) bool {
		return cg.ID == 1 && !cg.IsActive
	}), mock.Anything, mock.Anything).Return(nil)

	// Use a standard context for testing
	standardContext := context.Background()

	// PHASE 1: Create Combined Group
	t.Run("1. Create Combined Group", func(t *testing.T) {
		// Create test request
		combinedGroupReq := &CombinedGroupRequest{
			CombinedGroup: newCombinedGroup,
			GroupIDs:      []int64{1},
			SpecialistIDs: []int64{1},
		}
		body, _ := json.Marshal(combinedGroupReq)
		r := httptest.NewRequest("POST", "/combined", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")

		// Use the standard context
		r = r.WithContext(standardContext)

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.createCombinedGroup(w, r)

		// Check response
		assert.Equal(t, http.StatusCreated, w.Code)

		var responseCombinedGroup models2.CombinedGroup
		err := json.Unmarshal(w.Body.Bytes(), &responseCombinedGroup)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), responseCombinedGroup.ID)
		assert.Equal(t, "New Combined Group", responseCombinedGroup.Name)
		assert.Equal(t, 1, len(responseCombinedGroup.Groups))
		assert.Equal(t, 1, len(responseCombinedGroup.AccessSpecialists))
	})

	// PHASE 2: Add Group to Combined Group
	t.Run("2. Add Group to Combined Group", func(t *testing.T) {
		// Create add groups request
		groupsReq := &GroupIDsRequest{
			GroupIDs: []int64{2},
		}
		body, _ := json.Marshal(groupsReq)
		r := httptest.NewRequest("POST", "/combined/1/groups", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.addGroupsToCombinedGroup(w, r)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		var responseCombinedGroup models2.CombinedGroup
		err := json.Unmarshal(w.Body.Bytes(), &responseCombinedGroup)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(responseCombinedGroup.Groups))
		assert.Equal(t, int64(1), responseCombinedGroup.Groups[0].ID)
		assert.Equal(t, int64(2), responseCombinedGroup.Groups[1].ID)
	})

	// PHASE 3: Update Combined Group Specialists
	t.Run("3. Update Combined Group Specialists", func(t *testing.T) {
		// Create specialists update request
		specialistsReq := &SupervisorRequest{
			SupervisorIDs: []int64{1, 2},
		}
		body, _ := json.Marshal(specialistsReq)
		r := httptest.NewRequest("POST", "/combined/1/specialists", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.updateCombinedGroupSpecialists(w, r)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		var responseCombinedGroup models2.CombinedGroup
		err := json.Unmarshal(w.Body.Bytes(), &responseCombinedGroup)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(responseCombinedGroup.AccessSpecialists))
		assert.Equal(t, int64(1), responseCombinedGroup.AccessSpecialists[0].ID)
		assert.Equal(t, int64(2), responseCombinedGroup.AccessSpecialists[1].ID)
	})

	// PHASE 4: Deactivate (Delete) Combined Group
	t.Run("4. Deactivate Combined Group", func(t *testing.T) {
		r := httptest.NewRequest("DELETE", "/combined/1", nil)

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.deleteCombinedGroup(w, r)

		// Check response
		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	// Verify all expectations were met
	mockGroupStore.AssertExpectations(t)
}

// TestRoomMerge tests merging rooms to create a combined group
func TestRoomMerge(t *testing.T) {
	rs, mockGroupStore, _ := setupTestAPI()

	// Setup test data
	mergedCombinedGroup := &models2.CombinedGroup{
		ID:           1,
		Name:         "Room 101 + Room 102",
		IsActive:     true,
		AccessPolicy: "all",
		Groups: []*models2.Group{
			{ID: 1, Name: "Group 1"},
			{ID: 2, Name: "Group 2"},
		},
		CreatedAt: time.Now(),
	}

	mockGroupStore.On("MergeRooms", mock.Anything, int64(101), int64(102), "", (*time.Time)(nil), "all").Return(mergedCombinedGroup, nil)

	// Create test request
	mergeReq := &MergeRoomsRequest{
		SourceRoomID: 101,
		TargetRoomID: 102,
	}
	body, _ := json.Marshal(mergeReq)
	r := httptest.NewRequest("POST", "/merge-rooms", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.mergeRooms(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response["success"].(bool))

	combinedGroupMap := response["combined_group"].(map[string]interface{})
	assert.Equal(t, float64(1), combinedGroupMap["id"])
	assert.Equal(t, "Room 101 + Room 102", combinedGroupMap["name"])

	groups := combinedGroupMap["groups"].([]interface{})
	assert.Equal(t, 2, len(groups))

	mockGroupStore.AssertExpectations(t)
}

// TestGroupModels tests the helper functions in models.go
func TestGroupModels(t *testing.T) {
	// Test ValidateGroup
	t.Run("ValidateGroup", func(t *testing.T) {
		// Valid group
		group := &models2.Group{
			Name: "Test Group",
		}
		err := ValidateGroup(group)
		assert.NoError(t, err)

		// Invalid group - missing name
		invalidGroup := &models2.Group{}
		err = ValidateGroup(invalidGroup)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")

		// Nil group
		err = ValidateGroup(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "group cannot be nil")
	})

	// Test ValidateCombinedGroup
	t.Run("ValidateCombinedGroup", func(t *testing.T) {
		specificGroupID := int64(1)

		// Valid combined group
		combinedGroup := &models2.CombinedGroup{
			Name:         "Test Combined Group",
			AccessPolicy: "all",
		}
		err := ValidateCombinedGroup(combinedGroup)
		assert.NoError(t, err)

		// Valid combined group with specific access policy
		specificCombinedGroup := &models2.CombinedGroup{
			Name:            "Test Specific Group",
			AccessPolicy:    "specific",
			SpecificGroupID: &specificGroupID,
		}
		err = ValidateCombinedGroup(specificCombinedGroup)
		assert.NoError(t, err)

		// Invalid combined group - missing name
		invalidCombinedGroup := &models2.CombinedGroup{
			AccessPolicy: "all",
		}
		err = ValidateCombinedGroup(invalidCombinedGroup)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")

		// Invalid combined group - invalid access policy
		invalidAccessPolicyCombinedGroup := &models2.CombinedGroup{
			Name:         "Test Invalid Policy",
			AccessPolicy: "invalid_policy",
		}
		err = ValidateCombinedGroup(invalidAccessPolicyCombinedGroup)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "access policy must be one of")

		// Invalid combined group - specific policy without group ID
		invalidSpecificGroupCombinedGroup := &models2.CombinedGroup{
			Name:         "Test Invalid Specific",
			AccessPolicy: "specific",
		}
		err = ValidateCombinedGroup(invalidSpecificGroupCombinedGroup)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "specific group ID is required")
	})

	// Test helper functions
	t.Run("Helper Functions", func(t *testing.T) {
		studentID := int64(101)
		specialistID := int64(201)

		// Create test group
		group := &models2.Group{
			ID:               1,
			Name:             "Test Group",
			RepresentativeID: &studentID,
			Students: []models2.Student{
				{ID: studentID},
			},
			Supervisors: []*models2.PedagogicalSpecialist{
				{ID: specialistID},
			},
		}

		// Test IsGroupMember
		assert.True(t, IsGroupMember(group, studentID))
		assert.False(t, IsGroupMember(group, 999))

		// Test IsSpecialistSupervisorOfGroup
		assert.True(t, IsSpecialistSupervisorOfGroup(group, specialistID))
		assert.False(t, IsSpecialistSupervisorOfGroup(group, 999))

		// Create test combined group
		groupID := int64(1)
		specificGroupID := int64(2)

		combinedGroup := &models2.CombinedGroup{
			ID:              1,
			Name:            "Test Combined Group",
			AccessPolicy:    "all",
			SpecificGroupID: &specificGroupID,
			Groups: []*models2.Group{
				{ID: groupID},
			},
			AccessSpecialists: []*models2.PedagogicalSpecialist{
				{ID: specialistID},
			},
		}

		// Test HasGroupAccessToCombinedGroup
		assert.True(t, HasGroupAccessToCombinedGroup(combinedGroup, groupID))
		assert.False(t, HasGroupAccessToCombinedGroup(combinedGroup, 999))

		// Test HasSpecialistAccessToCombinedGroup
		assert.True(t, HasSpecialistAccessToCombinedGroup(combinedGroup, specialistID))
		assert.False(t, HasSpecialistAccessToCombinedGroup(combinedGroup, 999))
	})
}
