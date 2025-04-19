// Package rfid handles RFID tag communication from Raspberry Pi
package rfid

import (
	"context"
	"fmt"
	"github.com/moto-nrw/project-phoenix/logging"
	models2 "github.com/moto-nrw/project-phoenix/models"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/sirupsen/logrus"
	"github.com/uptrace/bun"
)

// API provides RFID handlers.
type API struct {
	store         RFIDStore
	userStore     UserStore
	studentStore  StudentStore
	timespanStore TimespanStore
}

// UserStore defines operations needed from the user store
type UserStore interface {
	GetCustomUserByTagID(ctx context.Context, tagID string) (*models2.CustomUser, error)
}

// StudentStore defines operations needed from the student store
type StudentStore interface {
	GetStudentByCustomUserID(ctx context.Context, customUserID int64) (*models2.Student, error)
	UpdateStudentLocation(ctx context.Context, id int64, locations map[string]bool) error
	ListStudents(ctx context.Context, filters map[string]interface{}) ([]models2.Student, error)
	CreateStudentVisit(ctx context.Context, studentID, roomID, timespanID int64) (*models2.Visit, error)
	GetStudentVisits(ctx context.Context, studentID int64, date *time.Time) ([]models2.Visit, error)
	GetRoomVisits(ctx context.Context, roomID int64, date *time.Time, active bool) ([]models2.Visit, error)
}

// TimespanStore defines operations needed from the timespan store
type TimespanStore interface {
	CreateTimespan(ctx context.Context, startTime time.Time, endTime *time.Time) (*models2.Timespan, error)
	GetTimespan(ctx context.Context, id int64) (*models2.Timespan, error)
	UpdateTimespanEndTime(ctx context.Context, id int64, endTime time.Time) error
}

// NewAPI configures and returns RFID API.
func NewAPI(db *bun.DB) (*API, error) {
	store := NewRFIDStore(db)
	api := &API{
		store: store,
	}
	return api, nil
}

// SetUserStore sets the user store for RFID API
func (a *API) SetUserStore(userStore UserStore) {
	a.userStore = userStore
}

// SetStudentStore sets the student store for RFID API
func (a *API) SetStudentStore(studentStore StudentStore) {
	a.studentStore = studentStore
}

// SetTimespanStore sets the timespan store for RFID API
func (a *API) SetTimespanStore(timespanStore TimespanStore) {
	a.timespanStore = timespanStore
}

// IsTestMode is a flag that can be set to disable authentication in tests
var IsTestMode bool = false

// apiKeyAuthMiddleware is a middleware to authenticate requests using API key
func (a *API) apiKeyAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.GetLogEntry(r)

		// Skip authentication in test mode
		if IsTestMode {
			// Create a fake test device for context
			testDevice := &TauriDevice{
				ID:          1,
				DeviceID:    "test-device-id",
				Name:        "Test Device",
				Description: "Test device for unit tests",
				Status:      "active",
				APIKey:      "test_api_key",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}

			// Add device to context
			ctx = context.WithValue(ctx, "device", testDevice)
			r = r.WithContext(ctx)

			// Proceed with request
			next.ServeHTTP(w, r)
			return
		}

		// Create a logger entry
		log := logrus.NewEntry(logrus.StandardLogger())
		if fieldLogger, ok := logger.(*logrus.Entry); ok {
			log = fieldLogger
		} else if logger != nil {
			log = logrus.NewEntry(logrus.StandardLogger()).WithFields(logrus.Fields{
				"request_id": middleware.GetReqID(ctx),
				"method":     r.Method,
				"path":       r.URL.Path,
			})
		}

		// Get API key from Authorization header
		apiKey := r.Header.Get("Authorization")
		if apiKey == "" {
			// Also check query string for API key (for GET requests only)
			if r.Method == "GET" {
				apiKey = r.URL.Query().Get("api_key")
			}
		}

		if apiKey == "" {
			log.Error("Missing API key")
			render.Render(w, r, ErrUnauthorized(fmt.Errorf("API key required")))
			return
		}

		// Strip "Bearer " prefix if present
		apiKey = strings.TrimPrefix(apiKey, "Bearer ")

		// Validate API key and get device
		device, err := a.store.GetDeviceByAPIKey(ctx, apiKey)
		if err != nil {
			log.WithError(err).Error("Invalid API key")
			render.Render(w, r, ErrUnauthorized(fmt.Errorf("invalid API key")))
			return
		}

		// Check if device is active
		if device.Status != "active" {
			log.WithField("device_id", device.DeviceID).Error("Inactive device attempted access")
			render.Render(w, r, ErrUnauthorized(fmt.Errorf("device is not active")))
			return
		}

		// Store device in request context
		ctx = context.WithValue(ctx, "device", device)
		r = r.WithContext(ctx)

		// Log the authenticated device access
		log.WithFields(logrus.Fields{
			"device_id":   device.DeviceID,
			"device_name": device.Name,
		}).Info("Authenticated API request")

		// Record last access IP
		ipAddress := r.RemoteAddr
		// Extract just the IP part if there's a port
		if strings.Contains(ipAddress, ":") {
			ipAddress = strings.Split(ipAddress, ":")[0]
		}

		// Update last IP, but don't fail the request if it doesn't work
		err = a.store.UpdateDevice(ctx, device.DeviceID, map[string]interface{}{
			"last_ip": ipAddress,
		})
		if err != nil {
			log.WithError(err).Warning("Failed to update device last_ip")
		}

		next.ServeHTTP(w, r)
	})
}

// Router provides RFID routes.
func (a *API) Router() *chi.Mux {
	r := chi.NewRouter()

	// Public endpoint for device registration - the only endpoint without auth
	r.Post("/devices", a.handleRegisterDevice)

	// Status endpoint has optional auth - works without auth but provides more details with auth
	r.Get("/app/status", a.handleTauriStatus)

	// API Key authentication middleware for all other endpoints
	r.Group(func(r chi.Router) {
		r.Use(a.apiKeyAuthMiddleware)

		// Endpoints for RFID Python Daemon
		r.Post("/tag", a.handleTagRead)
		r.Get("/tags", a.handleGetAllTags)

		// Student tracking with RFID
		r.Post("/track-student", a.handleStudentTracking)

		// Room occupancy tracking
		r.Post("/room-entry", a.handleRoomEntry)
		r.Post("/room-exit", a.handleRoomExit)
		r.Get("/room-occupancy", a.handleGetRoomOccupancy)

		// Student visit records
		r.Get("/student/{id}/visits", a.handleGetStudentVisits)
		r.Get("/room/{id}/visits", a.handleGetRoomVisits)
		r.Get("/visits/today", a.handleGetTodayVisits)

		// Endpoints for Tauri App
		r.Post("/app/sync", a.handleTauriSync)

		// Protected device management endpoints
		r.Route("/devices", func(r chi.Router) {
			r.Get("/", a.handleListDevices)
			r.Get("/{device_id}", a.handleGetDevice)
			r.Put("/{device_id}", a.handleUpdateDevice)
			r.Get("/{device_id}/sync-history", a.handleGetDeviceSyncHistory)
		})
	})

	return r
}

// handleTagRead processes RFID tag reads from the Raspberry Pi
func (a *API) handleTagRead(w http.ResponseWriter, r *http.Request) {
	data := &TagReadRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	tag, err := a.store.SaveTag(r.Context(), data.TagID, data.ReaderID)
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, tag)
}

// handleGetAllTags returns all stored RFID tags
func (a *API) handleGetAllTags(w http.ResponseWriter, r *http.Request) {
	tags, err := a.store.GetAllTags(r.Context())
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, tags)
}

// handleTauriSync processes synchronization requests from the Tauri app
func (a *API) handleTauriSync(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logging.GetLogEntry(r)

	// Create an entry from the logger
	log := logrus.NewEntry(logrus.StandardLogger())
	if fieldLogger, ok := logger.(*logrus.Entry); ok {
		log = fieldLogger
	} else if logger != nil {
		log = logrus.NewEntry(logrus.StandardLogger()).WithFields(logrus.Fields{
			"request_id": middleware.GetReqID(ctx),
			"method":     r.Method,
			"path":       r.URL.Path,
		})
	}

	// Get the device from context (set by middleware)
	device, ok := ctx.Value("device").(*TauriDevice)
	if !ok {
		log.Error("Device not found in context")
		render.Render(w, r, ErrInternalServer(fmt.Errorf("device authentication error")))
		return
	}

	// Parse request data
	data := &TauriSyncRequest{}
	if err := render.Bind(r, data); err != nil {
		log.WithError(err).Error("Failed to parse Tauri sync request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Ensure the device ID in the request matches the API key's device
	if data.DeviceID != device.DeviceID {
		log.WithFields(logrus.Fields{
			"request_device_id": data.DeviceID,
			"auth_device_id":    device.DeviceID,
		}).Error("Device ID mismatch")
		render.Render(w, r, ErrUnauthorized(fmt.Errorf("device ID mismatch")))
		return
	}

	log.WithFields(logrus.Fields{
		"device_id":   data.DeviceID,
		"tags_count":  len(data.Data),
		"app_version": data.AppVersion,
	}).Info("Processing Tauri sync request")

	// Save the tags from the Tauri app
	err := a.store.SaveTauriTags(ctx, data.DeviceID, data.Data)
	if err != nil {
		log.WithError(err).Error("Failed to save tags from Tauri app")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Record the sync event
	ipAddress := r.RemoteAddr
	// Extract just the IP part if there's a port
	if strings.Contains(ipAddress, ":") {
		ipAddress = strings.Split(ipAddress, ":")[0]
	}

	// Record sync regardless of success of student processing
	err = a.store.RecordDeviceSync(ctx, data.DeviceID, ipAddress, data.AppVersion, len(data.Data))
	if err != nil {
		log.WithError(err).Warning("Failed to record device sync")
		// Continue processing anyway
	}

	// Process student tracking for all tags
	processedCount := 0
	if a.userStore != nil && a.studentStore != nil {
		for _, tag := range data.Data {
			// Try to find the user by tag ID
			user, err := a.userStore.GetCustomUserByTagID(ctx, tag.TagID)
			if err != nil {
				log.WithFields(logrus.Fields{
					"tag_id": tag.TagID,
					"error":  err.Error(),
				}).Debug("No user found for tag")
				continue
			}

			// Get student information
			student, err := a.studentStore.GetStudentByCustomUserID(ctx, user.ID)
			if err != nil {
				log.WithFields(logrus.Fields{
					"user_id": user.ID,
					"error":   err.Error(),
				}).Debug("User found but no student record")
				continue
			}

			// Update student location based on reader type
			// This is a simplified version - in a real implementation, you would
			// determine the location type based on the reader information
			locationUpdates := make(map[string]bool)
			locationUpdates["in_house"] = true // Default for Tauri app is in-house

			err = a.studentStore.UpdateStudentLocation(ctx, student.ID, locationUpdates)
			if err != nil {
				log.WithError(err).Error("Failed to update student location")
				continue
			}

			processedCount++
			log.WithFields(logrus.Fields{
				"tag_id":     tag.TagID,
				"student_id": student.ID,
				"user_name":  user.FirstName + " " + user.SecondName,
			}).Info("Student location tracked via Tauri sync")
		}
	}

	// Return success response
	response := &TauriSyncResponse{
		Success: true,
		Message: fmt.Sprintf("Successfully synced %d tags, processed %d student locations",
			len(data.Data), processedCount),
	}

	render.JSON(w, r, response)
}

// handleTauriStatus returns status information to the Tauri app
func (a *API) handleTauriStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Get the logger
	logger := logging.GetLogEntry(r)

	// Create a logger entry
	log := logrus.NewEntry(logrus.StandardLogger())
	if fieldLogger, ok := logger.(*logrus.Entry); ok {
		log = fieldLogger
	} else if logger != nil {
		log = logrus.NewEntry(logrus.StandardLogger()).WithFields(logrus.Fields{
			"request_id": middleware.GetReqID(ctx),
			"method":     r.Method,
			"path":       r.URL.Path,
		})
	}

	// Check for API key in both query string and Authorization header
	apiKey := r.URL.Query().Get("api_key")
	if apiKey == "" {
		apiKey = r.Header.Get("Authorization")
		// Strip "Bearer " prefix if present
		apiKey = strings.TrimPrefix(apiKey, "Bearer ")
	}

	// Status requests can work without authentication, but if API key is provided, validate it
	var device *TauriDevice
	if apiKey != "" {
		var err error
		device, err = a.store.GetDeviceByAPIKey(ctx, apiKey)
		if err != nil {
			log.WithError(err).Warning("Invalid API key in status request")
			// Continue anyway, but don't include device-specific info
		} else if device.Status != "active" {
			log.WithField("device_id", device.DeviceID).Warning("Inactive device requested status")
			// Continue anyway, but don't include device-specific info
		}
	}

	log.Info("Processing Tauri app status request")

	// Get tag statistics
	tagCount, err := a.store.GetTagStats(ctx)
	if err != nil {
		log.WithError(err).Error("Failed to get tag statistics")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Get additional statistics - we can extend this as needed
	stats := AppStats{
		TagCount: tagCount,
	}

	// Add additional student statistics if studentStore is available
	if a.studentStore != nil {
		// For example: find how many students are currently in-house
		// This would require adding a CountStudentsByLocation method to the StudentStore
		// For now, we're just providing a placeholder
		stats.StudentsInHouse = 0
		stats.StudentsInWC = 0
		stats.StudentsInSchoolYard = 0

		// Try to get student locations from a filter-based query
		filters := map[string]interface{}{
			"in_house": true,
		}

		studentsInHouse, err := a.studentStore.ListStudents(ctx, filters)
		if err == nil {
			stats.StudentsInHouse = len(studentsInHouse)
		}

		// WC students
		filters = map[string]interface{}{
			"wc": true,
		}

		studentsInWC, err := a.studentStore.ListStudents(ctx, filters)
		if err == nil {
			stats.StudentsInWC = len(studentsInWC)
		}

		// Schoolyard students
		filters = map[string]interface{}{
			"school_yard": true,
		}

		studentsInSchoolYard, err := a.studentStore.ListStudents(ctx, filters)
		if err == nil {
			stats.StudentsInSchoolYard = len(studentsInSchoolYard)
		}
	}

	// Create status response
	status := &AppStatus{
		Status:    "ok",
		Timestamp: time.Now(),
		Stats:     stats,
		Version:   "1.0.0", // You might want to get this from your app configuration
	}

	// Record the status check if we have a valid device
	if device != nil {
		ipAddress := r.RemoteAddr
		// Extract just the IP part if there's a port
		if strings.Contains(ipAddress, ":") {
			ipAddress = strings.Split(ipAddress, ":")[0]
		}

		// Don't return error if this fails, just log it
		_ = a.store.UpdateDevice(ctx, device.DeviceID, map[string]interface{}{
			"last_ip": ipAddress,
		})
	}

	render.JSON(w, r, status)
}

// StudentTrackingRequest is the request for student tracking with RFID
type StudentTrackingRequest struct {
	TagID        string `json:"tag_id"`
	ReaderID     string `json:"reader_id"`
	LocationType string `json:"location_type"` // "entry", "wc", "schoolyard", or "exit"
}

// Bind preprocesses a StudentTrackingRequest
func (req *StudentTrackingRequest) Bind(r *http.Request) error {
	return nil
}

// StudentTrackingResponse is the response for student tracking
type StudentTrackingResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	StudentID int64  `json:"student_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Location  string `json:"location,omitempty"`
}

// handleRoomEntry processes a student entering a room with an RFID tag
func (a *API) handleRoomEntry(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Get the logger
	logger := logging.GetLogEntry(r)

	// Create a logger entry
	log := logrus.NewEntry(logrus.StandardLogger())
	if fieldLogger, ok := logger.(*logrus.Entry); ok {
		log = fieldLogger
	} else if logger != nil {
		log = logrus.NewEntry(logrus.StandardLogger()).WithFields(logrus.Fields{
			"request_id": middleware.GetReqID(ctx),
			"method":     r.Method,
			"path":       r.URL.Path,
		})
	}

	// Parse the request
	data := &RoomEntryRequest{}
	if err := render.Bind(r, data); err != nil {
		log.WithError(err).Error("Failed to parse room entry request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	log.WithFields(logrus.Fields{
		"tag_id":    data.TagID,
		"room_id":   data.RoomID,
		"reader_id": data.ReaderID,
	}).Info("Processing room entry request")

	// First log the tag read
	_, err := a.store.SaveTag(ctx, data.TagID, data.ReaderID)
	if err != nil {
		log.WithError(err).Error("Failed to save tag read")
		// Continue anyway, as this is just for logging
	}

	// Make sure we have the required stores
	if a.userStore == nil {
		render.Render(w, r, ErrInternalServer(fmt.Errorf("user store not configured")))
		return
	}

	// Find the user by tag ID
	user, err := a.userStore.GetCustomUserByTagID(ctx, data.TagID)
	if err != nil {
		log.WithFields(logrus.Fields{
			"tag_id": data.TagID,
			"error":  err.Error(),
		}).Info("Tag scanned but no user found")

		render.JSON(w, r, &OccupancyResponse{
			Success: false,
			Message: "No user found with this tag ID",
		})
		return
	}

	// Get the student associated with this user
	if a.studentStore == nil {
		render.Render(w, r, ErrInternalServer(fmt.Errorf("student store not configured")))
		return
	}

	student, err := a.studentStore.GetStudentByCustomUserID(ctx, user.ID)
	if err != nil {
		log.WithFields(logrus.Fields{
			"user_id": user.ID,
			"error":   err.Error(),
		}).Info("User found but no student record")

		render.JSON(w, r, &OccupancyResponse{
			Success: false,
			Message: "User found but no student record",
		})
		return
	}

	// Create a timespan for the visit
	var timespan *models2.Timespan
	if a.timespanStore != nil {
		// Create timespan with start time now and no end time
		timespan, err = a.timespanStore.CreateTimespan(ctx, time.Now(), nil)
		if err != nil {
			log.WithError(err).Error("Failed to create timespan for visit")
			render.Render(w, r, ErrInternalServer(err))
			return
		}

		// Create a proper visit record
		visit, err := a.studentStore.CreateStudentVisit(ctx, student.ID, data.RoomID, timespan.ID)
		if err != nil {
			log.WithError(err).Error("Failed to create student visit")
			// Don't return, we'll still update location
		} else {
			log.WithFields(logrus.Fields{
				"visit_id":    visit.ID,
				"student_id":  student.ID,
				"room_id":     data.RoomID,
				"timespan_id": timespan.ID,
			}).Info("Created visit record")
		}
	}

	// Record the room entry in the RFID system as well
	err = a.store.RecordRoomEntry(ctx, student.ID, data.RoomID)
	if err != nil {
		log.WithError(err).Error("Failed to record room entry")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Update student location to in_house
	locationUpdates := map[string]bool{
		"in_house":    true,
		"wc":          false,
		"school_yard": false,
	}

	err = a.studentStore.UpdateStudentLocation(ctx, student.ID, locationUpdates)
	if err != nil {
		log.WithError(err).Warning("Failed to update student location, but room entry was recorded")
	}

	// Get the updated room occupancy
	occupancy, err := a.store.GetRoomOccupancy(ctx, data.RoomID)
	studentCount := 0
	if err == nil {
		studentCount = occupancy.StudentCount
	}

	// Return success response
	response := &OccupancyResponse{
		Success:      true,
		Message:      "Student entered room successfully",
		StudentID:    student.ID,
		RoomID:       data.RoomID,
		StudentCount: studentCount,
	}

	log.WithFields(logrus.Fields{
		"student_id":    student.ID,
		"room_id":       data.RoomID,
		"student_count": studentCount,
	}).Info("Student entered room")

	render.JSON(w, r, response)
}

// handleRoomExit processes a student exiting a room with an RFID tag
func (a *API) handleRoomExit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Get the logger
	logger := logging.GetLogEntry(r)

	// Create a logger entry
	log := logrus.NewEntry(logrus.StandardLogger())
	if fieldLogger, ok := logger.(*logrus.Entry); ok {
		log = fieldLogger
	} else if logger != nil {
		log = logrus.NewEntry(logrus.StandardLogger()).WithFields(logrus.Fields{
			"request_id": middleware.GetReqID(ctx),
			"method":     r.Method,
			"path":       r.URL.Path,
		})
	}

	// Parse the request
	data := &RoomExitRequest{}
	if err := render.Bind(r, data); err != nil {
		log.WithError(err).Error("Failed to parse room exit request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	log.WithFields(logrus.Fields{
		"tag_id":    data.TagID,
		"room_id":   data.RoomID,
		"reader_id": data.ReaderID,
	}).Info("Processing room exit request")

	// First log the tag read
	_, err := a.store.SaveTag(ctx, data.TagID, data.ReaderID)
	if err != nil {
		log.WithError(err).Error("Failed to save tag read")
		// Continue anyway, as this is just for logging
	}

	// Make sure we have the required stores
	if a.userStore == nil {
		render.Render(w, r, ErrInternalServer(fmt.Errorf("user store not configured")))
		return
	}

	// Find the user by tag ID
	user, err := a.userStore.GetCustomUserByTagID(ctx, data.TagID)
	if err != nil {
		log.WithFields(logrus.Fields{
			"tag_id": data.TagID,
			"error":  err.Error(),
		}).Info("Tag scanned but no user found")

		render.JSON(w, r, &OccupancyResponse{
			Success: false,
			Message: "No user found with this tag ID",
		})
		return
	}

	// Get the student associated with this user
	if a.studentStore == nil {
		render.Render(w, r, ErrInternalServer(fmt.Errorf("student store not configured")))
		return
	}

	student, err := a.studentStore.GetStudentByCustomUserID(ctx, user.ID)
	if err != nil {
		log.WithFields(logrus.Fields{
			"user_id": user.ID,
			"error":   err.Error(),
		}).Info("User found but no student record")

		render.JSON(w, r, &OccupancyResponse{
			Success: false,
			Message: "User found but no student record",
		})
		return
	}

	// Find active visit records for this student in this room and end them
	if a.timespanStore != nil && a.studentStore != nil {
		// Get room visits that are active (with a focus on those with null end times)
		activeVisits, err := a.studentStore.GetRoomVisits(ctx, data.RoomID, nil, true)
		if err != nil {
			log.WithError(err).Warning("Failed to get active visits for this room")
		} else {
			// Look for this student's active visits
			for _, visit := range activeVisits {
				if visit.StudentID == student.ID && visit.Timespan != nil && visit.Timespan.EndTime == nil {
					// Update the visit timespan to end now
					err = a.timespanStore.UpdateTimespanEndTime(ctx, visit.TimespanID, time.Now())
					if err != nil {
						log.WithError(err).Error("Failed to update timespan end time")
					} else {
						log.WithFields(logrus.Fields{
							"visit_id":    visit.ID,
							"student_id":  student.ID,
							"room_id":     data.RoomID,
							"timespan_id": visit.TimespanID,
						}).Info("Ended visit record")
					}
				}
			}
		}
	}

	// Record the room exit in the RFID system as well
	err = a.store.RecordRoomExit(ctx, student.ID, data.RoomID)
	if err != nil {
		log.WithError(err).Error("Failed to record room exit")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Get the updated room occupancy
	occupancy, err := a.store.GetRoomOccupancy(ctx, data.RoomID)
	studentCount := 0
	if err == nil {
		studentCount = occupancy.StudentCount
	}

	// Return success response
	response := &OccupancyResponse{
		Success:      true,
		Message:      "Student exited room successfully",
		StudentID:    student.ID,
		RoomID:       data.RoomID,
		StudentCount: studentCount,
	}

	log.WithFields(logrus.Fields{
		"student_id":    student.ID,
		"room_id":       data.RoomID,
		"student_count": studentCount,
	}).Info("Student exited room")

	render.JSON(w, r, response)
}

// handleGetRoomOccupancy returns the current occupancy for a room
func (a *API) handleGetRoomOccupancy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Get the logger
	logger := logging.GetLogEntry(r)

	// Create a logger entry
	log := logrus.NewEntry(logrus.StandardLogger())
	if fieldLogger, ok := logger.(*logrus.Entry); ok {
		log = fieldLogger
	} else if logger != nil {
		log = logrus.NewEntry(logrus.StandardLogger()).WithFields(logrus.Fields{
			"request_id": middleware.GetReqID(ctx),
			"method":     r.Method,
			"path":       r.URL.Path,
		})
	}

	// Get the room ID from the query parameter
	roomIDStr := r.URL.Query().Get("room_id")
	var roomID int64 = 0
	var err error

	if roomIDStr != "" {
		roomID, err = strconv.ParseInt(roomIDStr, 10, 64)
		if err != nil {
			log.WithError(err).Error("Invalid room ID")
			render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid room ID: %s", roomIDStr)))
			return
		}
	}

	log.WithField("room_id", roomID).Info("Getting room occupancy")

	// If no room ID is provided, return all rooms
	if roomID == 0 {
		rooms, err := a.store.GetCurrentRooms(ctx)
		if err != nil {
			log.WithError(err).Error("Failed to get room occupancy")
			render.Render(w, r, ErrInternalServer(err))
			return
		}

		render.JSON(w, r, rooms)
		return
	}

	// Get occupancy for the specified room
	occupancy, err := a.store.GetRoomOccupancy(ctx, roomID)
	if err != nil {
		log.WithError(err).Error("Failed to get room occupancy")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, occupancy)
}

// handleGetStudentVisits returns visits for a specific student
func (a *API) handleGetStudentVisits(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get the logger
	logger := logging.GetLogEntry(r)
	log := logrus.NewEntry(logrus.StandardLogger())
	if fieldLogger, ok := logger.(*logrus.Entry); ok {
		log = fieldLogger
	} else if logger != nil {
		log = logrus.NewEntry(logrus.StandardLogger()).WithFields(logrus.Fields{
			"request_id": middleware.GetReqID(ctx),
			"method":     r.Method,
			"path":       r.URL.Path,
		})
	}

	// Get student ID from URL parameter
	studentIDStr := chi.URLParam(r, "id")
	studentID, err := strconv.ParseInt(studentIDStr, 10, 64)
	if err != nil {
		log.WithError(err).Error("Invalid student ID")
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid student ID: %s", studentIDStr)))
		return
	}

	// Check if date filter is provided
	dateStr := r.URL.Query().Get("date")
	var date *time.Time

	if dateStr != "" {
		parsedDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			log.WithError(err).Error("Invalid date format")
			render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid date format, use YYYY-MM-DD: %s", dateStr)))
			return
		}
		date = &parsedDate
	}

	// Get student visits
	if a.studentStore == nil {
		render.Render(w, r, ErrInternalServer(fmt.Errorf("student store not configured")))
		return
	}

	visits, err := a.studentStore.GetStudentVisits(ctx, studentID, date)
	if err != nil {
		log.WithError(err).Error("Failed to get student visits")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, visits)
}

// handleGetRoomVisits returns visits for a specific room
func (a *API) handleGetRoomVisits(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get the logger
	logger := logging.GetLogEntry(r)
	log := logrus.NewEntry(logrus.StandardLogger())
	if fieldLogger, ok := logger.(*logrus.Entry); ok {
		log = fieldLogger
	} else if logger != nil {
		log = logrus.NewEntry(logrus.StandardLogger()).WithFields(logrus.Fields{
			"request_id": middleware.GetReqID(ctx),
			"method":     r.Method,
			"path":       r.URL.Path,
		})
	}

	// Get room ID from URL parameter
	roomIDStr := chi.URLParam(r, "id")
	roomID, err := strconv.ParseInt(roomIDStr, 10, 64)
	if err != nil {
		log.WithError(err).Error("Invalid room ID")
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid room ID: %s", roomIDStr)))
		return
	}

	// Check if date filter is provided
	dateStr := r.URL.Query().Get("date")
	var date *time.Time

	if dateStr != "" {
		parsedDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			log.WithError(err).Error("Invalid date format")
			render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid date format, use YYYY-MM-DD: %s", dateStr)))
			return
		}
		date = &parsedDate
	}

	// Check if active filter is provided
	activeStr := r.URL.Query().Get("active")
	active := false
	if activeStr == "true" || activeStr == "1" {
		active = true
	}

	// Get room visits
	if a.studentStore == nil {
		render.Render(w, r, ErrInternalServer(fmt.Errorf("student store not configured")))
		return
	}

	visits, err := a.studentStore.GetRoomVisits(ctx, roomID, date, active)
	if err != nil {
		log.WithError(err).Error("Failed to get room visits")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, visits)
}

// handleGetTodayVisits returns all visits for today
func (a *API) handleGetTodayVisits(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get the logger
	logger := logging.GetLogEntry(r)
	log := logrus.NewEntry(logrus.StandardLogger())
	if fieldLogger, ok := logger.(*logrus.Entry); ok {
		log = fieldLogger
	} else if logger != nil {
		log = logrus.NewEntry(logrus.StandardLogger()).WithFields(logrus.Fields{
			"request_id": middleware.GetReqID(ctx),
			"method":     r.Method,
			"path":       r.URL.Path,
		})
	}

	// Use today's date
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Get active visits for all rooms
	if a.studentStore == nil {
		render.Render(w, r, ErrInternalServer(fmt.Errorf("student store not configured")))
		return
	}

	// For this endpoint, we'll check all rooms with active visits
	// First, let's get a list of all rooms with current occupancy
	roomsData, err := a.store.GetCurrentRooms(ctx)
	if err != nil {
		log.WithError(err).Error("Failed to get rooms with occupancy")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Get visits for each room
	allVisits := []models2.Visit{}
	for _, room := range roomsData {
		visits, err := a.studentStore.GetRoomVisits(ctx, room.RoomID, &today, false)
		if err != nil {
			log.WithFields(logrus.Fields{
				"room_id": room.RoomID,
				"error":   err.Error(),
			}).Warning("Failed to get visits for room")
			continue
		}
		allVisits = append(allVisits, visits...)
	}

	render.JSON(w, r, allVisits)
}

// handleRegisterDevice registers a new Tauri app device
func (a *API) handleRegisterDevice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logging.GetLogEntry(r)

	// Create a logger entry
	log := logrus.NewEntry(logrus.StandardLogger())
	if fieldLogger, ok := logger.(*logrus.Entry); ok {
		log = fieldLogger
	} else if logger != nil {
		log = logrus.NewEntry(logrus.StandardLogger()).WithFields(logrus.Fields{
			"request_id": middleware.GetReqID(ctx),
			"method":     r.Method,
			"path":       r.URL.Path,
		})
	}

	// Parse the request
	data := &DeviceRegisterRequest{}
	if err := render.Bind(r, data); err != nil {
		log.WithError(err).Error("Failed to parse device registration request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	log.WithFields(logrus.Fields{
		"device_id": data.DeviceID,
		"name":      data.Name,
	}).Info("Processing device registration request")

	// Register the device
	device, apiKey, err := a.store.RegisterDevice(ctx, data.DeviceID, data.Name, data.Description)
	if err != nil {
		log.WithError(err).Error("Failed to register device")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Return the registration response with the API key
	// Note: This is the only time the API key will be returned
	response := &DeviceRegisterResponse{
		Success:  true,
		Message:  "Device registered successfully",
		Device:   *device,
		APIKey:   apiKey,
		DeviceID: device.DeviceID,
	}

	log.WithField("device_id", device.DeviceID).Info("Device registered successfully")

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, response)
}

// handleGetDevice retrieves a device by ID
func (a *API) handleGetDevice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logging.GetLogEntry(r)

	// Create a logger entry
	log := logrus.NewEntry(logrus.StandardLogger())
	if fieldLogger, ok := logger.(*logrus.Entry); ok {
		log = fieldLogger
	} else if logger != nil {
		log = logrus.NewEntry(logrus.StandardLogger()).WithFields(logrus.Fields{
			"request_id": middleware.GetReqID(ctx),
			"method":     r.Method,
			"path":       r.URL.Path,
		})
	}

	// Get device ID from URL parameter
	deviceID := chi.URLParam(r, "device_id")
	if deviceID == "" {
		log.Error("Device ID is empty")
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("device ID is required")))
		return
	}

	log.WithField("device_id", deviceID).Info("Getting device")

	// Get the device
	device, err := a.store.GetDevice(ctx, deviceID)
	if err != nil {
		log.WithError(err).Error("Failed to get device")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, device)
}

// handleUpdateDevice updates a device's information
func (a *API) handleUpdateDevice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logging.GetLogEntry(r)

	// Create a logger entry
	log := logrus.NewEntry(logrus.StandardLogger())
	if fieldLogger, ok := logger.(*logrus.Entry); ok {
		log = fieldLogger
	} else if logger != nil {
		log = logrus.NewEntry(logrus.StandardLogger()).WithFields(logrus.Fields{
			"request_id": middleware.GetReqID(ctx),
			"method":     r.Method,
			"path":       r.URL.Path,
		})
	}

	// Get device ID from URL parameter
	deviceID := chi.URLParam(r, "device_id")
	if deviceID == "" {
		log.Error("Device ID is empty")
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("device ID is required")))
		return
	}

	// Parse the request
	data := &DeviceUpdateRequest{}
	if err := render.Bind(r, data); err != nil {
		log.WithError(err).Error("Failed to parse device update request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	log.WithFields(logrus.Fields{
		"device_id": deviceID,
	}).Info("Updating device")

	// Build updates map
	updates := make(map[string]interface{})
	if data.Name != "" {
		updates["name"] = data.Name
	}
	if data.Description != "" {
		updates["description"] = data.Description
	}
	if data.Status != "" {
		updates["status"] = data.Status
	}

	// Update the device
	if len(updates) > 0 {
		err := a.store.UpdateDevice(ctx, deviceID, updates)
		if err != nil {
			log.WithError(err).Error("Failed to update device")
			render.Render(w, r, ErrInternalServer(err))
			return
		}
	}

	// Get the updated device
	device, err := a.store.GetDevice(ctx, deviceID)
	if err != nil {
		log.WithError(err).Error("Failed to get updated device")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	log.WithField("device_id", deviceID).Info("Device updated successfully")

	render.JSON(w, r, device)
}

// handleListDevices retrieves all registered devices
func (a *API) handleListDevices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logging.GetLogEntry(r)

	// Create a logger entry
	log := logrus.NewEntry(logrus.StandardLogger())
	if fieldLogger, ok := logger.(*logrus.Entry); ok {
		log = fieldLogger
	} else if logger != nil {
		log = logrus.NewEntry(logrus.StandardLogger()).WithFields(logrus.Fields{
			"request_id": middleware.GetReqID(ctx),
			"method":     r.Method,
			"path":       r.URL.Path,
		})
	}

	log.Info("Getting all devices")

	// Get the devices
	devices, err := a.store.ListDevices(ctx)
	if err != nil {
		log.WithError(err).Error("Failed to get devices")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, devices)
}

// handleGetDeviceSyncHistory retrieves sync history for a device
func (a *API) handleGetDeviceSyncHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logging.GetLogEntry(r)

	// Create a logger entry
	log := logrus.NewEntry(logrus.StandardLogger())
	if fieldLogger, ok := logger.(*logrus.Entry); ok {
		log = fieldLogger
	} else if logger != nil {
		log = logrus.NewEntry(logrus.StandardLogger()).WithFields(logrus.Fields{
			"request_id": middleware.GetReqID(ctx),
			"method":     r.Method,
			"path":       r.URL.Path,
		})
	}

	// Get device ID from URL parameter
	deviceID := chi.URLParam(r, "device_id")
	if deviceID == "" {
		log.Error("Device ID is empty")
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("device ID is required")))
		return
	}

	// Get limit from query parameter
	limitStr := r.URL.Query().Get("limit")
	limit := 50 // Default limit
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			log.WithError(err).Error("Invalid limit")
			render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid limit: %s", limitStr)))
			return
		}
	}

	log.WithFields(logrus.Fields{
		"device_id": deviceID,
		"limit":     limit,
	}).Info("Getting device sync history")

	// Get the sync history
	history, err := a.store.GetDeviceSyncHistory(ctx, deviceID, limit)
	if err != nil {
		log.WithError(err).Error("Failed to get device sync history")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	render.JSON(w, r, history)
}

// handleStudentTracking processes RFID tags for student tracking
func (a *API) handleStudentTracking(w http.ResponseWriter, r *http.Request) {
	data := &StudentTrackingRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	// Get the logger - our improved GetLogEntry will handle all the edge cases
	logger := logging.GetLogEntry(r)

	// Create an entry from the logger - it's always safe to do this
	log := logrus.NewEntry(logrus.StandardLogger())
	if fieldLogger, ok := logger.(*logrus.Entry); ok {
		log = fieldLogger
	} else if logger != nil {
		// If it's not nil but not a *logrus.Entry, create a new entry that copies the fields
		log = logrus.NewEntry(logrus.StandardLogger()).WithFields(logrus.Fields{
			"request_id": middleware.GetReqID(ctx),
			"method":     r.Method,
			"path":       r.URL.Path,
		})
	}

	// Check if userStore is set
	if a.userStore == nil {
		render.Render(w, r, ErrInternalServer(fmt.Errorf("user store not configured")))
		return
	}

	// First log the tag read
	_, err := a.store.SaveTag(ctx, data.TagID, data.ReaderID)
	if err != nil {
		log.WithError(err).Error("Failed to save tag read")
		// Continue anyway, as this is just for logging
	}

	// Find the user by tag ID
	user, err := a.userStore.GetCustomUserByTagID(ctx, data.TagID)
	if err != nil {
		log.WithFields(logrus.Fields{
			"tag_id":    data.TagID,
			"reader_id": data.ReaderID,
			"error":     err.Error(),
		}).Info("Tag scanned but no user found")

		render.JSON(w, r, &StudentTrackingResponse{
			Success: false,
			Message: "No user found with this tag ID",
		})
		return
	}

	// Determine the location update
	locationUpdate := ""
	locationUpdates := make(map[string]bool)

	switch data.LocationType {
	case "entry":
		locationUpdate = "in-house"
		locationUpdates["in_house"] = true
		locationUpdates["wc"] = false
		locationUpdates["school_yard"] = false
	case "wc":
		locationUpdate = "bathroom"
		locationUpdates["in_house"] = true
		locationUpdates["wc"] = true
		locationUpdates["school_yard"] = false
	case "schoolyard":
		locationUpdate = "schoolyard"
		locationUpdates["in_house"] = false
		locationUpdates["wc"] = false
		locationUpdates["school_yard"] = true
	case "exit":
		locationUpdate = "out"
		locationUpdates["in_house"] = false
		locationUpdates["wc"] = false
		locationUpdates["school_yard"] = false
	default:
		locationUpdate = "unknown"
	}

	// Update student location if studentStore is configured
	studentID := int64(0)
	if a.studentStore != nil {
		student, err := a.studentStore.GetStudentByCustomUserID(ctx, user.ID)
		if err == nil {
			studentID = student.ID
			err = a.studentStore.UpdateStudentLocation(ctx, student.ID, locationUpdates)
			if err != nil {
				log.WithError(err).Error("Failed to update student location")
			}
		} else {
			log.WithError(err).WithField("user_id", user.ID).Warning("Found user but not student record")
		}
	}

	// Log the tracking event
	log.WithFields(logrus.Fields{
		"tag_id":     data.TagID,
		"reader_id":  data.ReaderID,
		"user_id":    user.ID,
		"user_name":  user.FirstName + " " + user.SecondName,
		"location":   data.LocationType,
		"student_id": studentID,
	}).Info("Student location tracked")

	// Return the tracking response
	render.JSON(w, r, &StudentTrackingResponse{
		Success:   true,
		Message:   "Location tracking recorded",
		StudentID: user.ID,
		Name:      user.FirstName + " " + user.SecondName,
		Location:  locationUpdate,
	})
}
