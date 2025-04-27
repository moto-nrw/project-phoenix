package room

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestGetRoomHistory tests the room history retrieval endpoint
func TestGetRoomHistory(t *testing.T) {
	api, mockStore := setupAPI(t)

	// Create test data
	room := &models.Room{ID: 1, RoomName: "Test Room"}
	history := []models.RoomHistory{
		{
			ID:             1,
			RoomID:         1,
			Room:           room,
			AgName:         "Test AG",
			Day:            time.Now(),
			TimespanID:     1,
			SupervisorID:   1,
			MaxParticipant: 10,
		},
	}

	// Configure mock store behaviors
	mockStore.On("GetRoomHistoryByRoom", mock.Anything, int64(1)).Return(history, nil)

	// Set up chi router with the history endpoint
	router := chi.NewRouter()
	router.Get("/{id}/history", api.handleGetRoomHistory)

	// Create HTTP request
	r := httptest.NewRequest("GET", "/1/history", nil)
	w := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(w, r)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var response []models.RoomHistory
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response, 1)
	assert.Equal(t, "Test AG", response[0].AgName)

	// Verify method calls
	mockStore.AssertExpectations(t)
}

// TestGetRoomHistoryByDateRange tests the room history by date range endpoint
func TestGetRoomHistoryByDateRange(t *testing.T) {
	api, mockStore := setupAPI(t)

	// Test dates
	startDate, _ := time.Parse("2006-01-02", "2023-01-01")
	endDate, _ := time.Parse("2006-01-02", "2023-01-31")
	endDatePlus1 := endDate.Add(24 * time.Hour)

	// Create test data
	history := []models.RoomHistory{
		{
			ID:             1,
			RoomID:         1,
			AgName:         "Test AG",
			Day:            startDate.Add(24 * time.Hour), // Jan 2nd
			TimespanID:     1,
			SupervisorID:   1,
			MaxParticipant: 10,
		},
	}

	// Configure mock store behaviors
	mockStore.On("GetRoomHistoryByDateRange", mock.Anything, startDate, endDatePlus1).Return(history, nil)

	// Set up chi router with the history endpoint
	router := chi.NewRouter()
	router.Get("/date", api.handleGetRoomHistoryByDateRange)

	// Create HTTP request
	r := httptest.NewRequest("GET", "/date?start_date=2023-01-01&end_date=2023-01-31", nil)
	w := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(w, r)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var response []models.RoomHistory
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response, 1)

	// Verify method calls
	mockStore.AssertExpectations(t)
}

// TestGetRoomHistoryBySupervisor tests the room history by supervisor endpoint
func TestGetRoomHistoryBySupervisor(t *testing.T) {
	api, mockStore := setupAPI(t)

	// Create test data
	specialist := &models.PedagogicalSpecialist{ID: 1}
	history := []models.RoomHistory{
		{
			ID:             1,
			RoomID:         1,
			AgName:         "Test AG",
			Day:            time.Now(),
			TimespanID:     1,
			SupervisorID:   1,
			Supervisor:     specialist,
			MaxParticipant: 10,
		},
	}

	// Configure mock store behaviors
	mockStore.On("GetRoomHistoryBySupervisor", mock.Anything, int64(1)).Return(history, nil)

	// Set up chi router with the history endpoint
	router := chi.NewRouter()
	router.Get("/supervisor/{id}", api.handleGetRoomHistoryBySupervisor)

	// Create HTTP request
	r := httptest.NewRequest("GET", "/supervisor/1", nil)
	w := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(w, r)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var response []models.RoomHistory
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response, 1)
	assert.Equal(t, int64(1), response[0].SupervisorID)

	// Verify method calls
	mockStore.AssertExpectations(t)
}
