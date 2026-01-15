package data

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	iotCommon "github.com/moto-nrw/project-phoenix/api/iot/common"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/users"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// getAvailableTeachers handles getting the list of teachers available for device login selection
// This endpoint only requires device authentication (no PIN required)
func (rs *Resource) getAvailableTeachers(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context (no staff context required)
	deviceCtx := device.DeviceFromCtx(r.Context())
	if deviceCtx == nil {
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Get all staff members who are teachers
	staffMembers, err := rs.UsersService.ListStaff(r.Context(), nil)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(err))
		return
	}

	// Build response with teachers who have PINs set
	responses := make([]DeviceTeacherResponse, 0)

	for _, staff := range staffMembers {
		// Check if this staff member is a teacher
		teacher, err := rs.UsersService.GetTeacherByStaffID(r.Context(), staff.ID)
		if err != nil || teacher == nil {
			continue // Skip non-teachers
		}

		// Get person details
		person, err := rs.UsersService.Get(r.Context(), staff.PersonID)
		if err != nil || person == nil {
			continue // Skip if person not found
		}

		// With global PIN, we no longer need to check individual PINs
		// All teachers are available for selection

		// Create teacher response
		response := DeviceTeacherResponse{
			StaffID:     staff.ID,
			PersonID:    person.ID,
			FirstName:   person.FirstName,
			LastName:    person.LastName,
			DisplayName: fmt.Sprintf("%s %s", person.FirstName, person.LastName),
		}

		responses = append(responses, response)
	}

	// Log device access for audit trail
	log.Printf("Device %s requested teacher list, returned %d teachers", deviceCtx.DeviceID, len(responses))

	common.Respond(w, r, http.StatusOK, responses, "Available teachers retrieved successfully")
}

// getTeacherStudents handles getting students supervised by authenticated teacher(s)
func (rs *Resource) getTeacherStudents(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
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
	response := rs.buildStudentResponses(r.Context(), uniqueStudents)

	common.Respond(w, r, http.StatusOK, response, fmt.Sprintf("Found %d unique students", len(response)))
}

// getTeacherActivities handles getting activities supervised by the authenticated teacher (for RFID devices)
func (rs *Resource) getTeacherActivities(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Get all activities without filtering by teacher
	activities, err := rs.ActivitiesService.ListGroups(r.Context(), nil)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(err))
		return
	}

	// Convert to response format
	response := make([]TeacherActivityResponse, 0, len(activities))
	for _, activity := range activities {
		categoryName := ""
		if activity.Category != nil {
			categoryName = activity.Category.Name
		}
		response = append(response, TeacherActivityResponse{
			ID:       activity.ID,
			Name:     activity.Name,
			Category: categoryName,
		})
	}

	common.Respond(w, r, http.StatusOK, response, "Activities fetched successfully")
}

// getAvailableRoomsForDevice handles getting available rooms for RFID devices
func (rs *Resource) getAvailableRoomsForDevice(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device and staff from context
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
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
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(err))
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
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Get tagId from URL parameter
	tagID := chi.URLParam(r, "tagId")
	if tagID == "" {
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(errors.New("tagId parameter is required")))
		return
	}

	// Normalize and find person by RFID tag
	normalizedTagID := iotCommon.NormalizeTagID(tagID)
	person := rs.findPersonByTag(r.Context(), normalizedTagID, tagID)

	// Build response based on person type
	response := rs.buildRFIDAssignmentResponse(r.Context(), person, normalizedTagID)

	common.Respond(w, r, http.StatusOK, response, "RFID tag assignment status retrieved")
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
			iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(errors.New("invalid teacher ID: "+idStr)))
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

// fetchStudentsForTeachers fetches unique students for all given teacher IDs
func (rs *Resource) fetchStudentsForTeachers(ctx context.Context, teacherIDs []int64) map[int64]usersSvc.StudentWithGroup {
	uniqueStudents := make(map[int64]usersSvc.StudentWithGroup)

	for _, staffID := range teacherIDs {
		teacher, err := rs.UsersService.GetTeacherByStaffID(ctx, staffID)
		if err != nil || teacher == nil {
			log.Printf("Error finding teacher for staff %d: %v", staffID, err)
			continue
		}

		students, err := rs.UsersService.GetStudentsWithGroupsByTeacher(ctx, teacher.ID)
		if err != nil {
			log.Printf("Error fetching students for teacher %d (staff %d): %v", teacher.ID, staffID, err)
			continue
		}

		for _, student := range students {
			uniqueStudents[student.Student.ID] = student
		}
	}

	return uniqueStudents
}

// buildStudentResponses builds response array from unique students map
func (rs *Resource) buildStudentResponses(ctx context.Context, uniqueStudents map[int64]usersSvc.StudentWithGroup) []TeacherStudentResponse {
	response := make([]TeacherStudentResponse, 0, len(uniqueStudents))

	for _, swg := range uniqueStudents {
		person, err := rs.UsersService.Get(ctx, swg.Student.PersonID)
		if err != nil {
			log.Printf("Error fetching person for student %d: %v", swg.Student.ID, err)
			continue
		}

		rfidTag := ""
		if person.TagID != nil {
			rfidTag = *person.TagID
		}

		response = append(response, TeacherStudentResponse{
			StudentID:   swg.Student.ID,
			PersonID:    swg.Student.PersonID,
			FirstName:   person.FirstName,
			LastName:    person.LastName,
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
		log.Printf("Warning: No person found for RFID tag %s: %v", originalTagID, err)
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
			log.Printf("Warning: Error finding student for person %d: %v", person.ID, err)
		}
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
			log.Printf("Warning: Error finding staff for person %d: %v", person.ID, err)
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
			log.Printf("Warning: Error checking teacher status for staff %d: %v", staffID, err)
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
