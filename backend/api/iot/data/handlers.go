package data

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	shared "github.com/moto-nrw/project-phoenix/api/iot/internal/shared"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// getAvailableTeachers handles getting the list of teachers available for device login selection
// This endpoint only requires device authentication (no PIN required)
func (rs *Resource) getAvailableTeachers(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context (no staff context required)
	deviceCtx := device.DeviceFromCtx(r.Context())
	if deviceCtx == nil {
		slog.WarnContext(r.Context(), "device auth missing API key", slog.String("path", r.URL.Path))
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Use the teacher roster directly so kiosk selection remains independent
	// of caregiver-account lifecycle state.
	teachers, err := rs.UsersService.ListTeachersWithStaffAndPerson(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	responses := make([]DeviceTeacherResponse, 0, len(teachers))
	for _, teacher := range teachers {
		if teacher == nil || teacher.Staff == nil || teacher.Staff.Person == nil {
			continue
		}

		person := teacher.Staff.Person
		responses = append(responses, DeviceTeacherResponse{
			StaffID:     teacher.StaffID,
			PersonID:    person.ID,
			FirstName:   person.FirstName,
			LastName:    person.LastName,
			DisplayName: strings.TrimSpace(person.FirstName + " " + person.LastName),
		})
	}

	// Log device access for audit trail
	slog.Default().InfoContext(r.Context(), "device requested teacher list",
		slog.String("device_id", deviceCtx.DeviceID),
		slog.Int("teacher_count", len(responses)),
	)

	common.Respond(w, r, http.StatusOK, responses, "Available teachers retrieved successfully")
}

// getTeacherStudents handles getting students supervised by authenticated teacher(s).
// When teacher_ids is omitted, returns ALL students (used for bracelet assignment).
func (rs *Resource) getTeacherStudents(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		slog.WarnContext(r.Context(), "device auth missing API key", slog.String("path", r.URL.Path))
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// If teacher_ids query key is absent entirely, return all students.
	// An explicitly empty value (?teacher_ids=) still goes through parseTeacherIDs
	// which returns an empty result — preserving previous behavior.
	if _, hasTeacherIDs := r.URL.Query()["teacher_ids"]; !hasTeacherIDs {
		rs.getAllStudents(w, r)
		return
	}

	// Parse teacher IDs from query parameters
	teacherIDs, ok := rs.parseTeacherIDs(w, r)
	if !ok {
		return
	}

	// Fetch unique students for all teachers
	uniqueStudents := rs.fetchStudentsForTeachers(r.Context(), teacherIDs)

	// Build response from unique students
	response := rs.buildStudentResponses(uniqueStudents)

	common.Respond(w, r, http.StatusOK, response, fmt.Sprintf("Found %d unique students", len(response)))
}

// getTeacherActivities handles getting activities supervised by the authenticated teacher (for RFID devices)
func (rs *Resource) getTeacherActivities(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		slog.WarnContext(r.Context(), "device auth missing API key", slog.String("path", r.URL.Path))
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Get all activities with occupancy status
	activitiesWithOccupancy, err := rs.ActivitiesService.ListGroupsWithOccupancy(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Convert to response format
	response := make([]TeacherActivityResponse, 0, len(activitiesWithOccupancy))
	for _, ag := range activitiesWithOccupancy {
		categoryName := ""
		if ag.Category != nil {
			categoryName = ag.Category.Name
		}
		response = append(response, TeacherActivityResponse{
			ID:         ag.ID,
			Name:       ag.Name,
			Category:   categoryName,
			IsOccupied: ag.IsOccupied,
		})
	}

	common.Respond(w, r, http.StatusOK, response, "Activities fetched successfully")
}

// getAvailableRoomsForDevice handles getting available rooms for RFID devices
func (rs *Resource) getAvailableRoomsForDevice(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device and staff from context
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		slog.WarnContext(r.Context(), "device auth missing API key", slog.String("path", r.URL.Path))
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Parse capacity parameter if provided
	capacity := 0
	if capacityStr := r.URL.Query().Get("capacity"); capacityStr != "" {
		if cap, err := strconv.Atoi(capacityStr); err == nil && cap > 0 {
			capacity = cap
		}
	}

	// Get available rooms with occupancy status from facility service
	roomsWithOccupancy, err := rs.FacilityService.GetAvailableRoomsWithOccupancy(r.Context(), capacity)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Convert to device response format
	responses := make([]DeviceRoomResponse, 0, len(roomsWithOccupancy))
	for _, roomWithOccupancy := range roomsWithOccupancy {
		response := newDeviceRoomResponse(roomWithOccupancy.Room)
		response.IsOccupied = roomWithOccupancy.IsOccupied
		responses = append(responses, response)
	}

	common.Respond(w, r, http.StatusOK, responses, "Available rooms retrieved successfully")
}

// checkRFIDTagAssignment handles checking if an RFID tag is assigned and to whom
func (rs *Resource) checkRFIDTagAssignment(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device and staff from context
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		slog.WarnContext(r.Context(), "device auth missing API key", slog.String("path", r.URL.Path))
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Get tagId from URL parameter
	tagID := chi.URLParam(r, "tagId")
	if tagID == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("tagId parameter is required")))
		return
	}

	// Normalize and find person by RFID tag
	normalizedTagID := users.NormalizeTagID(tagID)
	person := rs.findPersonByTag(r.Context(), normalizedTagID, tagID)

	// Build response based on person type
	response := rs.buildRFIDAssignmentResponse(r.Context(), person, normalizedTagID)

	common.Respond(w, r, http.StatusOK, response, "RFID tag assignment status retrieved")
}

// getAllStudents returns all students with group info (no teacher filter)
func (rs *Resource) getAllStudents(w http.ResponseWriter, r *http.Request) {
	allStudents, err := rs.UsersService.GetAllStudentsWithGroups(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	response := make([]TeacherStudentResponse, 0, len(allStudents))
	for _, swg := range allStudents {
		if swg.Student == nil || swg.Student.Person == nil {
			continue
		}

		rfidTag := ""
		if swg.Student.Person.TagID != nil {
			rfidTag = *swg.Student.Person.TagID
		}

		response = append(response, TeacherStudentResponse{
			StudentID:   swg.Student.ID,
			PersonID:    swg.Student.PersonID,
			FirstName:   swg.Student.Person.FirstName,
			LastName:    swg.Student.Person.LastName,
			SchoolClass: swg.Student.SchoolClass,
			GroupName:   swg.GroupName,
			RFIDTag:     rfidTag,
		})
	}

	common.Respond(w, r, http.StatusOK, response, fmt.Sprintf("Found %d students", len(response)))
}

// Helper functions

// parseTeacherIDs parses comma-separated teacher IDs from query parameter
func (rs *Resource) parseTeacherIDs(w http.ResponseWriter, r *http.Request) ([]int64, bool) {
	teacherIDsParam := r.URL.Query().Get("teacher_ids")
	if teacherIDsParam == "" {
		common.Respond(w, r, http.StatusOK, []TeacherStudentResponse{}, "No teacher IDs provided")
		return nil, false
	}

	teacherIDStrings := strings.Split(teacherIDsParam, ",")
	teacherIDs := make([]int64, 0, len(teacherIDStrings))
	for _, idStr := range teacherIDStrings {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid teacher ID: "+idStr)))
			return nil, false
		}
		teacherIDs = append(teacherIDs, id)
	}

	if len(teacherIDs) == 0 {
		common.Respond(w, r, http.StatusOK, []TeacherStudentResponse{}, "No valid teacher IDs provided")
		return nil, false
	}

	return teacherIDs, true
}

// fetchStudentsForTeachers fetches unique students for all given teacher IDs.
// Uses batch lookup to resolve staff IDs → teachers in a single query.
func (rs *Resource) fetchStudentsForTeachers(ctx context.Context, teacherIDs []int64) map[int64]usersSvc.StudentWithGroup {
	uniqueStudents := make(map[int64]usersSvc.StudentWithGroup)
	// Batch resolve all staff IDs → teachers in one query (WHERE staff_id IN (?))
	teacherMap, err := rs.UsersService.GetTeachersByStaffIDs(ctx, teacherIDs)
	if err != nil {
		slog.Default().WarnContext(ctx, "failed to batch-find teachers by staff IDs",
			slog.String("error", err.Error()),
		)
		return uniqueStudents
	}

	resolvedTeacherIDs := make([]int64, 0, len(teacherIDs))
	for _, staffID := range teacherIDs {
		teacher, ok := teacherMap[staffID]
		if !ok || teacher == nil {
			slog.Default().WarnContext(ctx, "no teacher found for staff",
				slog.Int64("staff_id", staffID),
			)
			continue
		}
		resolvedTeacherIDs = append(resolvedTeacherIDs, teacher.ID)
	}
	students, err := rs.UsersService.GetStudentsWithGroupsByTeachers(ctx, resolvedTeacherIDs)
	if err != nil {
		slog.Default().WarnContext(ctx, "failed to batch-find students for teachers", slog.String("error", err.Error()))
		return uniqueStudents
	}
	for _, student := range students {
		uniqueStudents[student.Student.ID] = student
	}

	return uniqueStudents
}

// buildStudentResponses builds response array from unique students map.
// Person data is already loaded via JOINed queries — no additional DB calls needed.
func (rs *Resource) buildStudentResponses(uniqueStudents map[int64]usersSvc.StudentWithGroup) []TeacherStudentResponse {
	response := make([]TeacherStudentResponse, 0, len(uniqueStudents))

	for _, swg := range uniqueStudents {
		if swg.Student == nil || swg.Student.Person == nil {
			continue
		}

		rfidTag := ""
		if swg.Student.Person.TagID != nil {
			rfidTag = *swg.Student.Person.TagID
		}

		response = append(response, TeacherStudentResponse{
			StudentID:   swg.Student.ID,
			PersonID:    swg.Student.PersonID,
			FirstName:   swg.Student.Person.FirstName,
			LastName:    swg.Student.Person.LastName,
			SchoolClass: swg.Student.SchoolClass,
			GroupName:   swg.GroupName,
			RFIDTag:     rfidTag,
		})
	}

	return response
}

// findPersonByTag finds a person by RFID tag ID with error handling
func (rs *Resource) findPersonByTag(ctx context.Context, normalizedTagID, originalTagID string) *users.Person {
	person, err := rs.UsersService.FindByTagID(ctx, normalizedTagID)
	if err != nil {
		shared.RecordUnregisteredTagScan(ctx, rs.UnregisteredTagScans, slog.Default(), normalizedTagID)
		slog.Default().WarnContext(ctx, "no person found for RFID tag",
			slog.String("tag_id", originalTagID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	return person
}

// buildRFIDAssignmentResponse builds RFID assignment response based on person type
func (rs *Resource) buildRFIDAssignmentResponse(ctx context.Context, person *users.Person, normalizedTagID string) RFIDTagAssignmentResponse {
	response := RFIDTagAssignmentResponse{Assigned: false}

	if person == nil || person.TagID == nil || *person.TagID != normalizedTagID {
		return response
	}

	fullName := person.FirstName + " " + person.LastName

	// Check if person is a student
	if studentResponse := rs.buildStudentRFIDResponse(ctx, person, fullName); studentResponse != nil {
		return *studentResponse
	}

	// Check if person is staff
	if staffResponse := rs.buildStaffRFIDResponse(ctx, person, fullName); staffResponse != nil {
		return *staffResponse
	}

	return response
}

// buildStudentRFIDResponse builds response if person is a student
func (rs *Resource) buildStudentRFIDResponse(ctx context.Context, person *users.Person, fullName string) *RFIDTagAssignmentResponse {
	student, err := rs.UsersService.GetStudentByPersonID(ctx, person.ID)
	if err != nil || student == nil {
		if err != nil {
			slog.Default().WarnContext(ctx, "error finding student for person",
				slog.Int64("person_id", person.ID),
				slog.String("error", err.Error()),
			)
		}
		return nil
	}

	// A graduated child is soft-deleted and absent from every device roster, so
	// reporting the tag as theirs would name a person the kiosk cannot act on:
	// the assignment view shows their name and class while the unassign call
	// 404s through the shared alumnus gate. Graduation releases the tag itself
	// (see GradeTransitionService.releaseGraduateTags), so this only covers rows
	// graduated before that existed — those bracelets read as free and can be
	// handed to a current child, which is what physically happened (#405 review).
	if student.Status == users.StudentStatusAlumnus {
		slog.Default().InfoContext(ctx, "rfid tag still bound to a graduated student, reporting as unassigned",
			slog.Int64("student_id", student.ID),
		)
		return nil
	}

	// Same for a child whose care has ended (#2487): the effect-day pass
	// releases the bracelet, so a tag still bound here predates that pass and
	// physically went back into the box.
	if student.CareEndedOn(timezone.TodayDate()) {
		rs.getLogger().InfoContext(ctx, "rfid tag still bound to a departed student, reporting as unassigned",
			slog.Int64("student_id", student.ID),
		)
		return nil
	}

	return &RFIDTagAssignmentResponse{
		Assigned:   true,
		PersonType: "student",
		Person: &RFIDTagAssignedPerson{
			ID:       student.ID,
			PersonID: person.ID,
			Name:     fullName,
			Group:    student.SchoolClass,
		},
		Student: &RFIDTagAssignedStudent{
			ID:    student.ID,
			Name:  fullName,
			Group: student.SchoolClass,
		},
	}
}

// buildStaffRFIDResponse builds response if person is staff
func (rs *Resource) buildStaffRFIDResponse(ctx context.Context, person *users.Person, fullName string) *RFIDTagAssignmentResponse {
	staff, err := rs.UsersService.GetStaffByPersonID(ctx, person.ID)
	if err != nil || staff == nil {
		if err != nil {
			slog.Default().WarnContext(ctx, "error finding staff for person",
				slog.Int64("person_id", person.ID),
				slog.String("error", err.Error()),
			)
		}
		return nil
	}

	groupInfo := rs.getStaffGroupInfo(ctx, staff.ID)

	return &RFIDTagAssignmentResponse{
		Assigned:   true,
		PersonType: "staff",
		Person: &RFIDTagAssignedPerson{
			ID:       staff.ID,
			PersonID: person.ID,
			Name:     fullName,
			Group:    groupInfo,
		},
	}
}

// getStaffGroupInfo gets role/group information for staff
func (rs *Resource) getStaffGroupInfo(ctx context.Context, staffID int64) string {
	teacher, err := rs.UsersService.GetTeacherByStaffID(ctx, staffID)
	if err != nil || teacher == nil {
		if err != nil {
			slog.Default().WarnContext(ctx, "error checking teacher status for staff",
				slog.Int64("staff_id", staffID),
				slog.String("error", err.Error()),
			)
		}
		return "Staff"
	}

	if teacher.Role != "" {
		return teacher.Role
	}
	if teacher.Specialization != "" {
		return teacher.Specialization
	}
	return "Teacher"
}

// newDeviceRoomResponse converts a facilities.Room to DeviceRoomResponse format
func newDeviceRoomResponse(room *facilities.Room) DeviceRoomResponse {
	return DeviceRoomResponse{
		ID:       room.ID,
		Name:     room.Name,
		Building: room.Building,
		Floor:    room.Floor,
		Capacity: room.Capacity,
		Category: room.Category,
		Color:    room.Color,
	}
}
