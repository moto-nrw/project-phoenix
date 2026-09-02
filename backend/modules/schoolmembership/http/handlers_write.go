package staff

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// createStaff creates a staff record, optionally with a teacher profile.
//
// The business rules of the create flow (adoption of an existing record,
// Lehrkraft vs caregiver profile) stay behind the injected CreateStaff
// closure; the adapter parses the body, checks that the person exists, and
// maps the outcome to the historical response contract.
func (rs *Resource) createStaff(w http.ResponseWriter, r *http.Request) {
	req := &StaffRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.failure(w, r, FailureInvalidRequest, err, "invalid_request")
		return
	}
	ctx := r.Context()

	person, err := rs.runtime.Person(ctx, req.PersonID.Int64())
	if err != nil {
		rs.failure(w, r, FailureNotFound, errors.New(msgPersonNotFound), "not_found")
		return
	}

	result, err := rs.runtime.CreateStaff(ctx, CreateStaffInput{
		PersonID:         req.PersonID.Int64(),
		StaffNotes:       req.StaffNotes,
		IsTeacher:        req.IsTeacher,
		Specialization:   req.Specialization,
		Role:             req.Role,
		Qualifications:   req.Qualifications,
		ActorPermissions: rs.runtime.Permissions(ctx),
	})
	if err != nil {
		rs.runtime.WriteFailure(w, r, err)
		return
	}

	isTeacher := result.Teacher != nil
	// A failed teacher record leaves the account on the plain-staff default
	// grant path only when the caller did not ask for a teacher at all.
	if person.AccountID != nil && (isTeacher || !result.TeacherCreationFailed) {
		rs.runtime.GrantDefaultPermissions(ctx, *person.AccountID, isTeacher)
	}

	access := rs.fieldAccess(ctx)
	switch {
	case result.TeacherCreationFailed:
		rs.respond(w, r, http.StatusCreated,
			buildStaffResponse(access, result.Staff, &person, false, enrichment{}),
			"Staff member created successfully, but failed to create teacher record")
	case isTeacher:
		rs.respond(w, r, http.StatusCreated,
			buildTeacherResponse(access, result.Staff, &person, *result.Teacher, enrichment{}),
			"Teacher created successfully")
	default:
		rs.respond(w, r, http.StatusCreated,
			buildStaffResponse(access, result.Staff, &person, false, enrichment{}),
			"Staff member created successfully")
	}
}

// updateStaff updates a staff record and its teacher profile.
func (rs *Resource) updateStaff(w http.ResponseWriter, r *http.Request) {
	staff, ok := rs.parseAndFindStaff(w, r)
	if !ok {
		return
	}
	req := &StaffRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.failure(w, r, FailureInvalidRequest, err, "invalid_request")
		return
	}
	ctx := r.Context()

	person, ok := rs.resolveUpdateTarget(w, r, staff, req.PersonID.Int64())
	if !ok {
		return
	}

	result, err := rs.runtime.UpdateStaff(ctx, UpdateStaffInput{
		StaffID:        staff.ID,
		PersonID:       req.PersonID.Int64(),
		StaffNotes:     req.StaffNotes,
		IsTeacher:      req.IsTeacher,
		Specialization: req.Specialization,
		Role:           req.Role,
		Qualifications: req.Qualifications,
	})
	if err != nil {
		rs.runtime.WriteFailure(w, r, err)
		return
	}

	response, message := rs.updateResponseFor(ctx, result, person)
	rs.respond(w, r, http.StatusOK, response, message)
}

// resolveUpdateTarget returns the person the updated record points at.
//
// Re-pointing a staff record at a different person moves the whole record —
// notes, teacher data, qualifications, time tracking — onto another human
// being. That is directory authority, not personnel-record maintenance, so
// staff:manage alone must not do it (#2906); the caller needs users:manage.
// Ordinary staff edits send the record's own person_id back unchanged and
// never reach the check.
func (rs *Resource) resolveUpdateTarget(w http.ResponseWriter, r *http.Request, staff schoolmembership.Staff, personID int64) (*Person, bool) {
	ctx := r.Context()
	if staff.PersonID == personID {
		person, err := rs.personFor(ctx, staff)
		if err != nil {
			rs.internal(w, r, err)
			return nil, false
		}
		return person, true
	}
	if !rs.runtime.HasPermission(permissions.UsersManage, rs.runtime.Permissions(ctx)) {
		rs.failure(w, r, FailureForbidden, errors.New("insufficient permission to reassign a staff record to another person"), "forbidden")
		return nil, false
	}
	person, err := rs.runtime.Person(ctx, personID)
	if err != nil {
		rs.failure(w, r, FailureNotFound, errors.New(msgPersonNotFound), "not_found")
		return nil, false
	}
	return &person, true
}

// updateResponseFor maps the teacher-record outcome of the update to the
// endpoint's historical response/message contract.
func (rs *Resource) updateResponseFor(ctx context.Context, result UpdateStaffResult, person *Person) (any, string) {
	access := rs.fieldAccess(ctx)
	switch result.Action {
	case TeacherActionUpdated, TeacherActionCreated, TeacherActionExisting:
		if result.Teacher != nil {
			return buildTeacherResponse(access, result.Staff, person, *result.Teacher, enrichment{}), "Teacher updated successfully"
		}
		return buildStaffResponse(access, result.Staff, person, false, enrichment{}), "Teacher updated successfully"
	case TeacherActionUpdateFailed:
		return buildStaffResponse(access, result.Staff, person, false, enrichment{}), "Staff member updated successfully, but failed to update teacher record"
	case TeacherActionCreateFailed:
		return buildStaffResponse(access, result.Staff, person, false, enrichment{}), "Staff member updated successfully, but failed to create teacher record"
	default:
		return buildStaffResponse(access, result.Staff, person, false, enrichment{}), "Staff member updated successfully"
	}
}

// deleteStaff offboards a staff member. Offboarding is idempotent: an ID
// that no longer exists still answers 200, as it always did.
func (rs *Resource) deleteStaff(w http.ResponseWriter, r *http.Request) {
	id, ok := rs.parseStaffID(w, r)
	if !ok {
		return
	}
	if err := rs.runtime.Offboard(r.Context(), id, rs.runtime.CurrentUsername(r.Context())); err != nil {
		rs.runtime.WriteFailure(w, r, err)
		return
	}
	rs.respond(w, r, http.StatusOK, nil, "Staff member deleted successfully")
}
