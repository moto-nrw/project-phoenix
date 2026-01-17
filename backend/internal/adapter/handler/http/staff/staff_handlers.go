package staff

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
)

// listStaff handles listing all staff members with optional filtering
func (rs *Resource) listStaff(w http.ResponseWriter, r *http.Request) {
	filters := parseListStaffFilters(r)
	ctx := r.Context()

	// Get all staff members with person data in a single query (avoids N+1)
	staffMembers, err := rs.PersonService.ListStaffWithPerson(ctx)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Collect all staff IDs for batch teacher lookup
	staffIDs := make([]int64, len(staffMembers))
	for i, s := range staffMembers {
		staffIDs[i] = s.ID
	}

	// Batch-load all teachers in a single query (avoids N+1)
	teacherMap, err := rs.PersonService.GetTeachersByStaffIDs(ctx, staffIDs)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build response objects using pre-loaded data
	responses := make([]interface{}, 0, len(staffMembers))
	for _, staff := range staffMembers {
		if response, include := rs.processStaffForListOptimized(ctx, staff, teacherMap, filters); include {
			responses = append(responses, response)
		}
	}

	common.Respond(w, r, http.StatusOK, responses, "Staff members retrieved successfully")
}

// getStaff handles getting a staff member by ID
func (rs *Resource) getStaff(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}

	// Get staff member with person data using service method
	staff, err := rs.PersonService.GetStaffWithPerson(r.Context(), id)
	if err != nil {
		if isNotFoundErr(err) {
			common.RenderError(w, r, ErrorNotFound(errors.New(common.MsgStaffNotFound)))
		} else {
			common.RenderError(w, r, ErrorInternalServer(err))
		}
		return
	}

	// If person data was not loaded by FindWithPerson, try to fetch it separately
	if staff.Person == nil && staff.PersonID > 0 {
		person, err := rs.PersonService.Get(r.Context(), staff.PersonID)
		if err != nil {
			if logger.Logger != nil {
				logger.Logger.WithField("staff_id", id).WithError(err).Warn("failed to get person data for staff member")
			}
			// Don't fail the request, just log the warning
		} else {
			staff.Person = person
		}
	}

	// Check if this staff member is also a teacher
	isTeacher := false
	var teacher *users.Teacher

	teacher, err = rs.PersonService.GetTeacherByStaffID(r.Context(), staff.ID)
	if err == nil && teacher != nil {
		// Create teacher response
		response := newTeacherResponse(staff, teacher)
		common.Respond(w, r, http.StatusOK, response, "Teacher retrieved successfully")
		return
	}

	// Create staff response
	response := newStaffResponse(staff, isTeacher)
	common.Respond(w, r, http.StatusOK, response, "Staff member retrieved successfully")
}

// createStaff handles creating a new staff member
func (rs *Resource) createStaff(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &StaffRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Verify person exists
	person, err := rs.PersonService.Get(r.Context(), req.PersonID)
	if err != nil {
		if isNotFoundErr(err) {
			common.RenderError(w, r, ErrorNotFound(errors.New("person not found")))
		} else {
			common.RenderError(w, r, ErrorInternalServer(err))
		}
		return
	}

	// Create staff
	staff := &users.Staff{
		PersonID:   req.PersonID,
		StaffNotes: req.StaffNotes,
	}

	// Create staff record
	if err := rs.PersonService.CreateStaff(r.Context(), staff); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Set person data for response
	staff.Person = person

	// If request indicates this is a teacher, create teacher record as well
	isTeacher := req.IsTeacher
	var teacher *users.Teacher

	if isTeacher {
		teacher = &users.Teacher{
			StaffID:        staff.ID,
			Specialization: strings.TrimSpace(req.Specialization),
			Role:           req.Role,
			Qualifications: req.Qualifications,
		}

		// Create teacher record
		if rs.PersonService.CreateTeacher(r.Context(), teacher) != nil {
			// Still return staff member even if teacher creation fails
			isTeacher = false
			response := newStaffResponse(staff, isTeacher)
			common.Respond(w, r, http.StatusCreated, response, "Staff member created successfully, but failed to create teacher record")
			return
		}

		// Grant groups:read permission to teacher if they have an account
		if person.AccountID != nil {
			rs.grantDefaultPermissions(r.Context(), *person.AccountID, "teacher")
		}

		// Return teacher response
		response := newTeacherResponse(staff, teacher)
		common.Respond(w, r, http.StatusCreated, response, "Teacher created successfully")
		return
	}

	// Grant groups:read permission to staff if they have an account
	if person.AccountID != nil {
		rs.grantDefaultPermissions(r.Context(), *person.AccountID, "staff")
	}

	// Return staff response
	response := newStaffResponse(staff, isTeacher)
	common.Respond(w, r, http.StatusCreated, response, "Staff member created successfully")
}

// updateStaff handles updating a staff member
func (rs *Resource) updateStaff(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}

	req := &StaffRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	staff, err := rs.PersonService.GetStaffByID(r.Context(), id)
	if err != nil {
		if isNotFoundErr(err) {
			common.RenderError(w, r, ErrorNotFound(errors.New(common.MsgStaffNotFound)))
		} else {
			common.RenderError(w, r, ErrorInternalServer(err))
		}
		return
	}

	// Update basic fields
	staff.StaffNotes = req.StaffNotes

	// Handle person ID change
	if staff.PersonID != req.PersonID {
		if err := rs.updateStaffPerson(r.Context(), staff, req.PersonID); err != nil {
			if isNotFoundErr(err) {
				common.RenderError(w, r, ErrorNotFound(errors.New("person not found")))
			} else {
				common.RenderError(w, r, ErrorInternalServer(err))
			}
			return
		}
	}

	// Update staff record
	if err := rs.PersonService.UpdateStaff(r.Context(), staff); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Reload staff with person data
	rs.reloadStaffWithPerson(r.Context(), staff, id)

	// Get existing teacher record if any
	teacher, _ := rs.PersonService.GetTeacherByStaffID(r.Context(), staff.ID)

	// Handle teacher record based on request
	response, message := rs.buildUpdateStaffResponse(r.Context(), staff, req, teacher)
	common.Respond(w, r, http.StatusOK, response, message)
}

// deleteStaff handles deleting a staff member
func (rs *Resource) deleteStaff(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}

	// Check if this staff member is also a teacher
	teacher, err := rs.PersonService.GetTeacherByStaffID(r.Context(), id)
	if err == nil && teacher != nil {
		// Delete teacher record first
		if rs.PersonService.DeleteTeacher(r.Context(), teacher.ID) != nil {
			common.RenderError(w, r, ErrorInternalServer(errors.New("failed to delete teacher record")))
			return
		}
	}

	// Delete staff member
	if err := rs.PersonService.DeleteStaff(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Staff member deleted successfully")
}

// getStaffGroups handles getting groups for a staff member
func (rs *Resource) getStaffGroups(w http.ResponseWriter, r *http.Request) {
	// Parse and get staff
	staff, ok := rs.parseAndGetStaff(w, r)
	if !ok {
		return
	}

	// Check if this staff member is a teacher
	teacher, err := rs.PersonService.GetTeacherByStaffID(r.Context(), staff.ID)
	if err != nil || teacher == nil {
		// If not a teacher, return empty groups list
		common.Respond(w, r, http.StatusOK, []GroupResponse{}, "Staff member is not a teacher and has no assigned groups")
		return
	}

	// Check if we have a reference to the Education service
	if rs.EducationService == nil {
		// If not, return an error
		common.RenderError(w, r, ErrorInternalServer(errors.New("education service not available")))
		return
	}

	// Get groups for this teacher
	groups, err := rs.EducationService.GetTeacherGroups(r.Context(), teacher.ID)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build response
	responses := make([]GroupResponse, 0, len(groups))
	for _, group := range groups {
		responses = append(responses, GroupResponse{
			ID:   group.ID,
			Name: group.Name,
		})
	}

	common.Respond(w, r, http.StatusOK, responses, "Teacher groups retrieved successfully")
}

// getAvailableStaff handles getting available staff members (teachers) for assignments
func (rs *Resource) getAvailableStaff(w http.ResponseWriter, r *http.Request) {
	// Get all staff members
	staffMembers, err := rs.PersonService.ListStaff(r.Context(), nil)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build response objects - only include staff who are teachers
	responses := make([]TeacherResponse, 0)

	for _, staff := range staffMembers {
		// Check if this staff member is a teacher
		teacher, err := rs.PersonService.GetTeacherByStaffID(r.Context(), staff.ID)
		if err != nil || teacher == nil {
			continue
		}

		// Get associated person
		person, err := rs.PersonService.Get(r.Context(), staff.PersonID)
		if err != nil {
			// Skip this staff member if person not found
			continue
		}

		// Set person data
		staff.Person = person

		// Create teacher response
		responses = append(responses, newTeacherResponse(staff, teacher))
	}

	common.Respond(w, r, http.StatusOK, responses, "Available staff members retrieved successfully")
}

// getStaffByRole handles GET /api/staff/by-role?role=user
// Returns staff members filtered by account role (useful for group transfer dropdowns)
func (rs *Resource) getStaffByRole(w http.ResponseWriter, r *http.Request) {
	roleName := r.URL.Query().Get("role")
	if roleName == "" {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("role parameter is required")))
		return
	}

	staff, err := rs.PersonService.ListStaff(r.Context(), nil)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	results := rs.filterStaffByRole(r.Context(), staff, roleName)
	common.Respond(w, r, http.StatusOK, results, "Staff members with role retrieved successfully")
}
