package iot

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/api/iot/attendance"
	iotCommon "github.com/moto-nrw/project-phoenix/api/iot/common"
	dataAPI "github.com/moto-nrw/project-phoenix/api/iot/data"
	"github.com/moto-nrw/project-phoenix/api/iot/devices"
	feedbackAPI "github.com/moto-nrw/project-phoenix/api/iot/feedback"
	rfidAPI "github.com/moto-nrw/project-phoenix/api/iot/rfid"
	sessionsAPI "github.com/moto-nrw/project-phoenix/api/iot/sessions"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	facilitiesSvc "github.com/moto-nrw/project-phoenix/services/facilities"
	feedbackSvc "github.com/moto-nrw/project-phoenix/services/feedback"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)


// ServiceDependencies groups all service dependencies for the IoT resource
type ServiceDependencies struct {
	IoTService        iotSvc.Service
	UsersService      usersSvc.PersonService
	ActiveService     activeSvc.Service
	ActivitiesService activitiesSvc.ActivityService
	ConfigService     configSvc.Service
	FacilityService   facilitiesSvc.Service
	EducationService  educationSvc.Service
	FeedbackService   feedbackSvc.Service
}

// Resource defines the IoT API resource
type Resource struct {
	IoTService        iotSvc.Service
	UsersService      usersSvc.PersonService
	ActiveService     activeSvc.Service
	ActivitiesService activitiesSvc.ActivityService
	ConfigService     configSvc.Service
	FacilityService   facilitiesSvc.Service
	EducationService  educationSvc.Service
	FeedbackService   feedbackSvc.Service
}

// NewResource creates a new IoT resource
func NewResource(deps ServiceDependencies) *Resource {
	return &Resource{
		IoTService:        deps.IoTService,
		UsersService:      deps.UsersService,
		ActiveService:     deps.ActiveService,
		ActivitiesService: deps.ActivitiesService,
		ConfigService:     deps.ConfigService,
		FacilityService:   deps.FacilityService,
		EducationService:  deps.EducationService,
		FeedbackService:   deps.FeedbackService,
	}
}

// Router returns a configured router for IoT endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth, _ := jwt.NewTokenAuth()

	// Public routes (if any device endpoints should be public)
	r.Group(func(r chi.Router) {
		// Some basic device info might be public
		// Currently no public routes for IoT devices
	})

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// Mount devices sub-router (handles device CRUD and admin operations)
		// All device routes require JWT authentication with IOT permissions
		devicesResource := devices.NewResource(rs.IoTService)
		r.Mount("/", devicesResource.Router())
	})

	// Device-only authenticated routes (API key only, no PIN required)
	r.Group(func(r chi.Router) {
		r.Use(device.DeviceOnlyAuthenticator(rs.IoTService))

		// Mount data sub-router for teachers endpoint (device-only auth)
		dataResource := dataAPI.NewResource(rs.IoTService, rs.UsersService, rs.ActivitiesService, rs.FacilityService)
		r.Mount("/", dataResource.TeachersRouter())
	})

	// Device-authenticated routes for RFID devices
	r.Group(func(r chi.Router) {
		r.Use(device.DeviceAuthenticator(rs.IoTService, rs.UsersService))

		// Device endpoints that require device API key + staff PIN authentication
		r.Post("/ping", rs.devicePing)
		r.Post("/checkin", rs.deviceCheckin)
		r.Get("/status", rs.deviceStatus)

		// Mount feedback sub-router (handles device-based feedback submission)
		feedbackResource := feedbackAPI.NewResource(rs.IoTService, rs.UsersService, rs.FeedbackService)
		r.Mount("/", feedbackResource.Router())

		// Mount data sub-router for device data queries (device + PIN auth)
		dataResourceAuth := dataAPI.NewResource(rs.IoTService, rs.UsersService, rs.ActivitiesService, rs.FacilityService)
		r.Mount("/", dataResourceAuth.Router())

		// Mount attendance sub-router (handles daily attendance tracking)
		attendanceResource := attendance.NewResource(rs.UsersService, rs.ActiveService, rs.EducationService)
		r.Mount("/attendance", attendanceResource.Router())

		// Mount sessions sub-router (handles activity session management and timeout)
		sessionsResource := sessionsAPI.NewResource(
			rs.IoTService,
			rs.UsersService,
			rs.ActiveService,
			rs.ActivitiesService,
			rs.ConfigService,
			rs.FacilityService,
			rs.EducationService,
		)
		r.Mount("/session", sessionsResource.Router())

		// Mount RFID sub-router (handles RFID tag assignment/unassignment for staff)
		rfidResource := rfidAPI.NewResource(rs.UsersService)
		r.Mount("/staff", rfidResource.Router())
	})

	return r
}










// Device-authenticated handlers for RFID devices

// devicePing handles ping requests from RFID devices
// This endpoint keeps both the device AND any active session alive
func (rs *Resource) devicePing(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context (no staff context needed with global PIN)
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Update device last seen time (already done in middleware, but let's be explicit)
	if err := rs.IoTService.PingDevice(r.Context(), deviceCtx.DeviceID); err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	// Also update session activity if device has an active session
	// This keeps the session alive as long as the device is pinging
	sessionActive := false
	if session, err := rs.ActiveService.GetDeviceCurrentSession(r.Context(), deviceCtx.ID); err == nil && session != nil {
		sessionActive = true // Session exists - set immediately regardless of update success
		if err := rs.ActiveService.UpdateSessionActivity(r.Context(), session.ID); err != nil {
			log.Printf("Warning: Failed to update session activity for session %d during ping: %v", session.ID, err)
		}
	}

	// Return device status (no staff info with global PIN)
	response := map[string]interface{}{
		"device_id":      deviceCtx.DeviceID,
		"device_name":    deviceCtx.Name,
		"status":         deviceCtx.Status,
		"last_seen":      deviceCtx.LastSeen,
		"is_online":      deviceCtx.IsOnline(),
		"ping_time":      time.Now(),
		"session_active": sessionActive,
	}

	common.Respond(w, r, http.StatusOK, response, "Device ping successful")
}

// deviceStatus handles status requests from RFID devices
func (rs *Resource) deviceStatus(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Return detailed device status
	response := map[string]interface{}{
		"device": map[string]interface{}{
			"id":          deviceCtx.ID,
			"device_id":   deviceCtx.DeviceID,
			"device_type": deviceCtx.DeviceType,
			"name":        deviceCtx.Name,
			"status":      deviceCtx.Status,
			"last_seen":   deviceCtx.LastSeen,
			"is_online":   deviceCtx.IsOnline(),
			"is_active":   deviceCtx.IsActive(),
		},
		"authenticated_at": time.Now(),
	}

	common.Respond(w, r, http.StatusOK, response, "Device status retrieved")
}

// CheckinRequest represents a student check-in request from RFID devices
type CheckinRequest struct {
	StudentRFID string `json:"student_rfid"`
	Action      string `json:"action"` // "checkin" or "checkout"
	RoomID      *int64 `json:"room_id,omitempty"`
}

// CheckinResponse represents the response to a student check-in request
type CheckinResponse struct {
	StudentID   int64     `json:"student_id"`
	StudentName string    `json:"student_name"`
	Action      string    `json:"action"`
	VisitID     *int64    `json:"visit_id,omitempty"`
	RoomName    string    `json:"room_name,omitempty"`
	ProcessedAt time.Time `json:"processed_at"`
	Message     string    `json:"message"`
	Status      string    `json:"status"`
}


// Bind validates the checkin request
func (req *CheckinRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.StudentRFID, validation.Required),
		// Note: Action field is ignored in logic but still required for API compatibility
		validation.Field(&req.Action, validation.Required, validation.In("checkin", "checkout")),
	)
}

// getStudentDailyCheckoutTime parses the daily checkout time from environment variable
func getStudentDailyCheckoutTime() (time.Time, error) {
	checkoutTimeStr := os.Getenv("STUDENT_DAILY_CHECKOUT_TIME")
	if checkoutTimeStr == "" {
		checkoutTimeStr = "15:00" // Default to 3:00 PM
	}

	// Parse time in HH:MM format
	parts := strings.Split(checkoutTimeStr, ":")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid checkout time format: %s", checkoutTimeStr)
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return time.Time{}, fmt.Errorf("invalid hour in checkout time: %s", checkoutTimeStr)
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return time.Time{}, fmt.Errorf("invalid minute in checkout time: %s", checkoutTimeStr)
	}

	now := time.Now()
	checkoutTime := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	return checkoutTime, nil
}

// ensureSchulhofRoom finds or creates the Schulhof room
func (rs *Resource) ensureSchulhofRoom(ctx context.Context) (*facilities.Room, error) {
	// Try to find existing Schulhof room
	room, err := rs.FacilityService.FindRoomByName(ctx, constants.SchulhofRoomName)
	if err == nil && room != nil {
		log.Printf("%s Found existing room: ID=%d", constants.SchulhofLogPrefix, room.ID)
		return room, nil
	}

	// Room not found - create it
	log.Printf("%s Room not found, auto-creating...", constants.SchulhofLogPrefix)

	capacity := constants.SchulhofRoomCapacity
	category := constants.SchulhofCategoryName
	color := constants.SchulhofColor

	newRoom := &facilities.Room{
		Name:     constants.SchulhofRoomName,
		Capacity: &capacity,
		Category: &category,
		Color:    &color,
	}

	if err := rs.FacilityService.CreateRoom(ctx, newRoom); err != nil {
		return nil, fmt.Errorf("failed to auto-create Schulhof room: %w", err)
	}

	log.Printf("%s Successfully auto-created room: ID=%d", constants.SchulhofLogPrefix, newRoom.ID)
	return newRoom, nil
}

// ensureSchulhofCategory finds or creates the Schulhof activity category
func (rs *Resource) ensureSchulhofCategory(ctx context.Context) (*activities.Category, error) {
	// Try to find existing Schulhof category
	categories, err := rs.ActivitiesService.ListCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list activity categories: %w", err)
	}

	for _, cat := range categories {
		if cat.Name == constants.SchulhofCategoryName {
			log.Printf("%s Found existing category: ID=%d", constants.SchulhofLogPrefix, cat.ID)
			return cat, nil
		}
	}

	// Category not found - create it
	log.Printf("%s Category not found, auto-creating...", constants.SchulhofLogPrefix)

	newCategory := &activities.Category{
		Name:        constants.SchulhofCategoryName,
		Description: constants.SchulhofCategoryDescription,
		Color:       constants.SchulhofColor,
	}

	createdCategory, err := rs.ActivitiesService.CreateCategory(ctx, newCategory)
	if err != nil {
		return nil, fmt.Errorf("failed to auto-create Schulhof category: %w", err)
	}

	log.Printf("%s Successfully auto-created category: ID=%d", constants.SchulhofLogPrefix, createdCategory.ID)
	return createdCategory, nil
}

// schulhofActivityGroup finds or creates the permanent Schulhof activity group.
// This function implements lazy initialization - it will auto-create the Schulhof
// infrastructure (room, category, activity) on first use if not found.
func (rs *Resource) schulhofActivityGroup(ctx context.Context) (*activities.Group, error) {
	// Build filter for Schulhof activity using constant
	// Use qualified column name to avoid ambiguity with category.name
	options := base.NewQueryOptions()
	filter := base.NewFilter()
	filter.Equal("group.name", constants.SchulhofActivityName)
	options.Filter = filter

	// Query activities service
	groups, err := rs.ActivitiesService.ListGroups(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to query Schulhof activity: %w", err)
	}

	// If activity exists, return it
	if len(groups) > 0 {
		log.Printf("%s Found existing activity: ID=%d", constants.SchulhofLogPrefix, groups[0].ID)
		return groups[0], nil
	}

	// Activity not found - auto-create the entire Schulhof infrastructure
	log.Printf("%s Activity not found, auto-creating infrastructure...", constants.SchulhofLogPrefix)

	// Step 1: Ensure Schulhof room exists
	room, err := rs.ensureSchulhofRoom(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure Schulhof room: %w", err)
	}

	// Step 2: Ensure Schulhof category exists
	category, err := rs.ensureSchulhofCategory(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure Schulhof category: %w", err)
	}

	// Step 3: Create the Schulhof activity group
	newActivity := &activities.Group{
		Name:            constants.SchulhofActivityName,
		MaxParticipants: constants.SchulhofMaxParticipants,
		IsOpen:          true, // Open activity - anyone can join
		CategoryID:      category.ID,
		PlannedRoomID:   &room.ID,
	}

	// CreateGroup requires supervisorIDs and schedules - pass empty slices for auto-created activity
	createdActivity, err := rs.ActivitiesService.CreateGroup(ctx, newActivity, []int64{}, []*activities.Schedule{})
	if err != nil {
		return nil, fmt.Errorf("failed to auto-create Schulhof activity: %w", err)
	}

	log.Printf("%s Successfully auto-created infrastructure: room=%d, category=%d, activity=%d",
		constants.SchulhofLogPrefix, room.ID, category.ID, createdActivity.ID)

	return createdActivity, nil
}

// deviceCheckin handles student check-in/check-out requests from RFID devices
func (rs *Resource) deviceCheckin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := time.Now()

	// Step 1: Validate device context
	deviceCtx := validateDeviceContext(w, r)
	if deviceCtx == nil {
		return
	}
	log.Printf("[CHECKIN] Starting process - Device: %s (ID: %d)", deviceCtx.DeviceID, deviceCtx.ID)

	// Step 2: Parse and validate request
	req := parseCheckinRequest(w, r, deviceCtx.DeviceID)
	if req == nil {
		return
	}
	log.Printf("[CHECKIN] Request details: action='%s', student_rfid='%s', room_id=%v", req.Action, req.StudentRFID, req.RoomID)

	// Step 3: Lookup person by RFID
	person := rs.lookupPersonByRFID(ctx, w, r, req.StudentRFID)
	if person == nil {
		return
	}

	// Step 4: Check if person is a student
	student := rs.lookupStudentFromPerson(ctx, person.ID)
	if student == nil {
		// Not a student - attempt staff scan handling (always return after)
		rs.handleStaffScan(w, r, deviceCtx, person)
		return
	}
	log.Printf("[CHECKIN] Found student: ID %d, Class: %s", student.ID, student.SchoolClass)
	student.Person = person

	// Step 5: Load current visit with room information
	currentVisit := rs.loadCurrentVisitWithRoom(ctx, student.ID)

	// Step 6: Process checkout if student has active visit
	var checkoutVisitID *int64
	var previousRoomName string
	var checkedOut bool

	// Step 6: Check for pending daily checkout (must check BEFORE processing checkout)
	if currentVisit != nil && rs.isPendingDailyCheckoutScenario(ctx, student, currentVisit) {
		handlePendingDailyCheckoutResponse(w, r, student, person, currentVisit)
		return
	}

	// Step 6b: Process checkout if student has active visit
	if currentVisit != nil {
		var err error
		checkoutVisitID, previousRoomName, err = rs.processCheckout(ctx, w, r, student, person, currentVisit)
		if err != nil {
			return
		}
		checkedOut = true
	}

	// Step 7: Determine if checkin should be skipped (same room scenario)
	skipCheckin := shouldSkipCheckin(req.RoomID, checkedOut, currentVisit)
	if skipCheckin {
		log.Printf("[CHECKIN] Student checked out from room %d, same as checkin room - skipping re-checkin", *req.RoomID)
	}

	// Step 8: Process checkin if room_id provided and not skipping
	checkinResult := rs.processStudentCheckin(ctx, w, r, student, person, &checkinProcessingInput{
		RoomID:       req.RoomID,
		SkipCheckin:  skipCheckin,
		CheckedOut:   checkedOut,
		CurrentVisit: currentVisit,
	})
	if checkinResult.Error != nil {
		return
	}
	newVisitID := checkinResult.NewVisitID
	roomName := checkinResult.RoomName

	// Step 9: Check for daily checkout scenario
	result := buildCheckinResult(&checkinResultInput{
		Student:          student,
		Person:           person,
		CheckedOut:       checkedOut,
		NewVisitID:       newVisitID,
		CheckoutVisitID:  checkoutVisitID,
		RoomName:         roomName,
		PreviousRoomName: previousRoomName,
		CurrentVisit:     currentVisit,
	})
	if result.Action == "" {
		// No action occurred - shouldn't happen but handle gracefully
		log.Printf("[CHECKIN] WARNING: No action determined for student %d", student.ID)
		result.Action = "no_action"
		result.GreetingMsg = "Keine Aktion durchgeführt"
	}

	// Step 10: Check daily checkout with education group
	if rs.shouldUpgradeToDailyCheckout(ctx, result.Action, student, currentVisit) {
		result.Action = "checked_out_daily"
	}

	// Step 11: Update session activity for device monitoring
	if req.RoomID != nil {
		rs.updateSessionActivityForDevice(ctx, *req.RoomID, deviceCtx.ID)
	}

	// Step 12: Build and send response
	response := buildCheckinResponse(student, result, now)
	log.Printf("[CHECKIN] Final response: action='%s', student='%s %s', message='%s', visit_id=%v, room='%s'",
		result.Action, person.FirstName, person.LastName, result.GreetingMsg, result.VisitID, result.RoomName)

	sendCheckinResponse(w, r, response, result.Action)
}

// shouldUpgradeToDailyCheckout checks if a checkout should be upgraded to daily checkout.
// Encapsulates the complex condition to reduce cognitive complexity in deviceCheckin.
func (rs *Resource) shouldUpgradeToDailyCheckout(ctx context.Context, action string, student *users.Student, currentVisit *active.Visit) bool {
	if action != "checked_out" {
		return false
	}
	if student.GroupID == nil || currentVisit == nil || currentVisit.ActiveGroup == nil {
		return false
	}
	return rs.shouldShowDailyCheckoutWithGroup(ctx, student, currentVisit)
}

// shouldShowDailyCheckoutWithGroup checks if daily checkout should be shown by verifying education group room
func (rs *Resource) shouldShowDailyCheckoutWithGroup(ctx context.Context, student *users.Student, currentVisit *active.Visit) bool {
	if student.GroupID == nil {
		return false
	}

	checkoutTime, err := getStudentDailyCheckoutTime()
	if err != nil || !time.Now().After(checkoutTime) {
		return false
	}

	educationGroup, err := rs.EducationService.GetGroup(ctx, *student.GroupID)
	if err != nil || educationGroup == nil || educationGroup.RoomID == nil {
		return false
	}

	return currentVisit.ActiveGroup.RoomID == *educationGroup.RoomID
}

// isPendingDailyCheckoutScenario checks if this scan should trigger a pending daily checkout
// (deferred checkout that waits for user confirmation before processing).
// This is called BEFORE processCheckout() to determine if we should return early.
func (rs *Resource) isPendingDailyCheckoutScenario(ctx context.Context, student *users.Student, currentVisit *active.Visit) bool {
	// Check prerequisites
	if student.GroupID == nil || currentVisit == nil || currentVisit.ActiveGroup == nil {
		return false
	}

	// Check if time has passed daily checkout threshold
	checkoutTime, err := getStudentDailyCheckoutTime()
	if err != nil || !time.Now().After(checkoutTime) {
		return false
	}

	// Check if student's room matches education group room
	educationGroup, err := rs.EducationService.GetGroup(ctx, *student.GroupID)
	if err != nil || educationGroup == nil || educationGroup.RoomID == nil {
		return false
	}

	return currentVisit.ActiveGroup.RoomID == *educationGroup.RoomID
}

// handlePendingDailyCheckoutResponse sends the pending daily checkout response and returns true if handled.
// This helper reduces cognitive complexity in deviceCheckin by extracting the response building logic.
func handlePendingDailyCheckoutResponse(w http.ResponseWriter, r *http.Request, student *users.Student, person *users.Person, currentVisit *active.Visit) {
	log.Printf("[CHECKIN] Pending daily checkout for student %s %s (ID: %d) - awaiting confirmation",
		person.FirstName, person.LastName, student.ID)

	// Get room name for response
	roomName := getRoomNameFromVisit(currentVisit)

	// Build and send pending response
	response := map[string]interface{}{
		"student_id":   student.ID,
		"student_name": person.FirstName + " " + person.LastName,
		"action":       "pending_daily_checkout",
		"visit_id":     currentVisit.ID,
		"room_name":    roomName,
		"processed_at": time.Now(),
		"message":      "Gehst du nach Hause?",
		"status":       "success",
	}
	sendCheckinResponse(w, r, response, "pending_daily_checkout")
}

// getRoomNameFromVisit extracts the room name from a visit's active group if available.
func getRoomNameFromVisit(visit *active.Visit) string {
	if visit != nil && visit.ActiveGroup != nil && visit.ActiveGroup.Room != nil {
		return visit.ActiveGroup.Room.Name
	}
	return ""
}





