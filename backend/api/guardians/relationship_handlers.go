package guardians

import (
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	guardianSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// getStudentGuardians handles getting all guardians for a student (PUBLIC - everyone can view for emergency)
func (rs *Resource) getStudentGuardians(w http.ResponseWriter, r *http.Request) {
	// Parse student ID from URL
	studentID, err := common.ParseIDParam(r, "studentId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return
	}

	// Get guardians with relationships
	guardiansWithRel, err := rs.GuardianService.GetStudentGuardians(r.Context(), studentID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Convert to response format
	responses := make([]*GuardianWithRelationship, 0, len(guardiansWithRel))
	for _, gwr := range guardiansWithRel {
		responses = append(responses, &GuardianWithRelationship{
			Guardian:           newGuardianResponse(gwr.Profile),
			RelationshipID:     gwr.Relationship.ID,
			RelationshipType:   gwr.Relationship.RelationshipType,
			IsPrimary:          gwr.Relationship.IsPrimary,
			IsEmergencyContact: gwr.Relationship.IsEmergencyContact,
			CanPickup:          gwr.Relationship.CanPickup,
			PickupNotes:        gwr.Relationship.PickupNotes,
			EmergencyPriority:  gwr.Relationship.EmergencyPriority,
		})
	}

	common.Respond(w, r, http.StatusOK, responses, "Student guardians retrieved successfully")
}

// getGuardianStudents handles getting all students for a guardian
func (rs *Resource) getGuardianStudents(w http.ResponseWriter, r *http.Request) {
	// Parse guardian ID from URL
	guardianID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errInvalidGuardianID)))
		return
	}

	// Get students with relationships
	studentsWithRel, err := rs.GuardianService.GetGuardianStudents(r.Context(), guardianID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Convert to response format
	responses := make([]*StudentWithRelationship, 0, len(studentsWithRel))
	for _, swr := range studentsWithRel {
		// Get person data for student
		person, err := rs.PersonService.Get(r.Context(), swr.Student.PersonID)
		if err != nil {
			log.Printf("Failed to get person for student %d: %v", swr.Student.ID, err)
			continue
		}

		responses = append(responses, &StudentWithRelationship{
			StudentID:          swr.Student.ID,
			FirstName:          person.FirstName,
			LastName:           person.LastName,
			SchoolClass:        swr.Student.SchoolClass,
			RelationshipID:     swr.Relationship.ID,
			RelationshipType:   swr.Relationship.RelationshipType,
			IsPrimary:          swr.Relationship.IsPrimary,
			IsEmergencyContact: swr.Relationship.IsEmergencyContact,
			CanPickup:          swr.Relationship.CanPickup,
			PickupNotes:        swr.Relationship.PickupNotes,
			EmergencyPriority:  swr.Relationship.EmergencyPriority,
		})
	}

	common.Respond(w, r, http.StatusOK, responses, "Guardian students retrieved successfully")
}

// linkGuardianToStudent handles linking a guardian to a student (SUPERVISOR only)
func (rs *Resource) linkGuardianToStudent(w http.ResponseWriter, r *http.Request) {
	// Parse student ID from URL
	studentID, err := common.ParseIDParam(r, "studentId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return
	}

	// Check permissions - only supervisors of the student's group can link guardians
	canModify, err := rs.canModifyStudent(r.Context(), studentID)
	if !canModify {
		common.RenderError(w, r, common.ErrorForbidden(err))
		return
	}

	// Parse request
	req := &StudentGuardianLinkRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Convert to service request
	linkReq := guardianSvc.StudentGuardianCreateRequest{
		StudentID:          studentID,
		GuardianProfileID:  req.GuardianProfileID,
		RelationshipType:   req.RelationshipType,
		IsPrimary:          req.IsPrimary,
		IsEmergencyContact: req.IsEmergencyContact,
		CanPickup:          req.CanPickup,
		PickupNotes:        req.PickupNotes,
		EmergencyPriority:  req.EmergencyPriority,
	}

	// Link guardian to student
	relationship, err := rs.GuardianService.LinkGuardianToStudent(r.Context(), linkReq)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, relationship, "Guardian linked to student successfully")
}

// updateStudentGuardianRelationship handles updating a student-guardian relationship (SUPERVISOR only)
func (rs *Resource) updateStudentGuardianRelationship(w http.ResponseWriter, r *http.Request) {
	// Parse relationship ID from URL
	relationshipID, err := common.ParseIDParam(r, "relationshipId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid relationship ID")))
		return
	}

	// Get the relationship to find the student ID
	relationship, err := rs.GuardianService.GetStudentGuardianRelationship(r.Context(), relationshipID)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("relationship not found")))
		return
	}

	// Check permissions - only supervisors of the student's group can update relationships
	canModify, err := rs.canModifyStudent(r.Context(), relationship.StudentID)
	if !canModify {
		common.RenderError(w, r, common.ErrorForbidden(err))
		return
	}

	// Parse request
	req := &StudentGuardianUpdateRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Convert to service request
	updateReq := guardianSvc.StudentGuardianUpdateRequest{
		RelationshipType:   req.RelationshipType,
		IsPrimary:          req.IsPrimary,
		IsEmergencyContact: req.IsEmergencyContact,
		CanPickup:          req.CanPickup,
		PickupNotes:        req.PickupNotes,
		EmergencyPriority:  req.EmergencyPriority,
	}

	// Update relationship
	if err := rs.GuardianService.UpdateStudentGuardianRelationship(r.Context(), relationshipID, updateReq); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Relationship updated successfully")
}

// removeGuardianFromStudent handles removing a guardian from a student (SUPERVISOR only)
func (rs *Resource) removeGuardianFromStudent(w http.ResponseWriter, r *http.Request) {
	// Parse student ID from URL
	studentID, err := common.ParseIDParam(r, "studentId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return
	}

	// Parse guardian ID from URL
	guardianID, err := common.ParseIDParam(r, "guardianId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errInvalidGuardianID)))
		return
	}

	// Check permissions - only supervisors of the student's group can remove guardians
	canModify, err := rs.canModifyStudent(r.Context(), studentID)
	if !canModify {
		common.RenderError(w, r, common.ErrorForbidden(err))
		return
	}

	// Remove guardian from student
	if err := rs.GuardianService.RemoveGuardianFromStudent(r.Context(), studentID, guardianID); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Guardian removed from student successfully")
}
