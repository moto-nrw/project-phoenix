package user

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/moto-nrw/project-phoenix/models"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestAPIEndpointsDirectly tests the API endpoints by calling the handler functions directly
func TestAPIEndpointsDirectly(t *testing.T) {
	// Setup mocks and API resource
	rs, mockUserStore, _ := setupTestAPI()

	// ======= Test Public Endpoints =======

	// 1. Test listUsersPublic
	users := []models.CustomUser{
		{ID: 1, FirstName: "John", SecondName: "Doe", TagID: strPtr("ABC123")},
		{ID: 2, FirstName: "Jane", SecondName: "Smith", TagID: strPtr("DEF456")},
	}

	mockUserStore.On("ListCustomUsers", mock.Anything).Return(users, nil).Once()

	req1 := httptest.NewRequest("GET", "/public/users", nil)
	w1 := httptest.NewRecorder()
	rs.listUsersPublic(w1, req1)

	assert.Equal(t, http.StatusOK, w1.Code)
	var publicUsers []map[string]interface{}
	json.Unmarshal(w1.Body.Bytes(), &publicUsers)
	assert.Equal(t, 2, len(publicUsers))
	assert.Equal(t, "John", publicUsers[0]["first_name"])

	// 2. Test listSpecialistsPublic
	specialistUser1 := &models.CustomUser{ID: 1, FirstName: "Teacher", SecondName: "One"}
	specialistUser2 := &models.CustomUser{ID: 2, FirstName: "Teacher", SecondName: "Two"}

	specialists := []models.PedagogicalSpecialist{
		{ID: 1, Role: "Teacher", UserID: 1, CustomUser: specialistUser1},
		{ID: 2, Role: "Principal", UserID: 2, CustomUser: specialistUser2},
	}

	mockUserStore.On("ListSpecialists", mock.Anything).Return(specialists, nil).Once()

	req2 := httptest.NewRequest("GET", "/public/supervisors", nil)
	w2 := httptest.NewRecorder()
	rs.listSupervisorsPublic(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)
	var publicSupervisors []map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &publicSupervisors)
	assert.Equal(t, 2, len(publicSupervisors))
	assert.Equal(t, "Teacher", publicSupervisors[0]["role"])

	// ======= Test Protected Endpoints =======

	// 3. Test createUser
	newUser := models.CustomUser{
		FirstName:  "New",
		SecondName: "User",
	}

	mockUserStore.On("CreateCustomUser", mock.Anything, mock.MatchedBy(func(u *models.CustomUser) bool {
		return u.FirstName == "New" && u.SecondName == "User"
	})).Return(nil).Once()

	userReqBody, _ := json.Marshal(UserRequest{CustomUser: &newUser})
	req3 := httptest.NewRequest("POST", "/users", bytes.NewReader(userReqBody))
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	rs.createUser(w3, req3)

	assert.Equal(t, http.StatusCreated, w3.Code)

	// 4. Test changeTagID
	mockUserStore.On("UpdateTagID", mock.Anything, int64(1), "NEW-TAG-123").Return(nil).Once()

	tagChangeReqBody, _ := json.Marshal(ChangeTagIDRequest{UserID: 1, TagID: "NEW-TAG-123"})
	req4 := httptest.NewRequest("POST", "/change-tag-id", bytes.NewReader(tagChangeReqBody))
	req4.Header.Set("Content-Type", "application/json")
	w4 := httptest.NewRecorder()
	rs.changeTagID(w4, req4)

	assert.Equal(t, http.StatusOK, w4.Code)
	var changeResponse map[string]bool
	json.Unmarshal(w4.Body.Bytes(), &changeResponse)
	assert.True(t, changeResponse["success"])

	// 5. Test processTagScan
	userWithTag := &models.CustomUser{
		ID:         3,
		FirstName:  "RFID",
		SecondName: "User",
		TagID:      strPtr("SCAN-TAG-123"),
		CreatedAt:  time.Now(),
		ModifiedAt: time.Now(),
	}

	mockUserStore.On("GetCustomUserByTagID", mock.Anything, "SCAN-TAG-123").Return(userWithTag, nil).Once()

	scanReqBody, _ := json.Marshal(TagScanRequest{TagID: "SCAN-TAG-123", DeviceID: "TEST-DEVICE"})
	req5 := httptest.NewRequest("POST", "/process-tag-scan", bytes.NewReader(scanReqBody))
	req5.Header.Set("Content-Type", "application/json")
	w5 := httptest.NewRecorder()

	// Create a context with request (normally provided by middleware)
	ctx := context.Background()
	req5 = req5.WithContext(ctx)

	rs.processTagScan(w5, req5)

	assert.Equal(t, http.StatusOK, w5.Code)
	var scanResponse map[string]interface{}
	json.Unmarshal(w5.Body.Bytes(), &scanResponse)
	assert.True(t, scanResponse["success"].(bool))
	assert.Equal(t, float64(3), scanResponse["user_id"].(float64))

	// Verify all mocks were called as expected
	mockUserStore.AssertExpectations(t)
}
