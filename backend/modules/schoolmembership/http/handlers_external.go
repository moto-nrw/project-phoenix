package staff

import (
	"context"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// externalStaffResponse is the staff response of the external-caregiver
// writes. InvitationToken is set for the local demo seeder only.
type externalStaffResponse struct {
	StaffResponse
	InvitationToken string `json:"invitation_token,omitempty"`
}

// createExternalStaff records an external caregiver and invites them to a
// moto account in the school role „Extern“ (#3823, #4573). The workflow
// writes person, staff member, caregiver profile and external caregiver record and the
// invitation in one savepoint, so a refused invitation leaves nothing behind.
func (rs *Resource) createExternalStaff(w http.ResponseWriter, r *http.Request) {
	req := &ExternalStaffRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.failure(w, r, FailureInvalidRequest, err, "invalid_request")
		return
	}
	ctx := r.Context()
	result, err := rs.runtime.CreateExternalCaregiver(ctx, ExternalCaregiverInput{
		FirstName: req.FirstName, LastName: req.LastName, Email: req.Email, Organization: req.Organization,
		ActorAccountID: rs.runtime.CurrentAccountID(ctx), ActorPermissions: rs.runtime.Permissions(ctx),
	})
	if err != nil {
		rs.runtime.ExternalFailure(w, r, err)
		return
	}
	rs.respondExternal(w, r, http.StatusCreated, result, "Externe Betreuungskraft eingetragen und eingeladen")
}

// inviteExternalStaff („Konto einladen“) invites an existing external
// caregiver without an account, also again or to another address; earlier
// open invitations of the person are revoked (#4573).
func (rs *Resource) inviteExternalStaff(w http.ResponseWriter, r *http.Request) {
	id, ok := rs.parseStaffID(w, r)
	if !ok {
		return
	}
	req := &ExternalInvitationRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.failure(w, r, FailureInvalidRequest, err, "invalid_request")
		return
	}
	ctx := r.Context()
	result, err := rs.runtime.InviteExternalCaregiver(ctx, ExternalInvitationInput{
		StaffID: id, Email: req.Email,
		ActorAccountID: rs.runtime.CurrentAccountID(ctx), ActorPermissions: rs.runtime.Permissions(ctx),
	})
	if err != nil {
		rs.runtime.ExternalFailure(w, r, err)
		return
	}
	rs.respondExternal(w, r, http.StatusOK, result, "Einladung verschickt")
}

// respondExternal answers with the staff response of the external caregiver
// the workflow just invited: its invitation is open by construction.
func (rs *Resource) respondExternal(w http.ResponseWriter, r *http.Request, status int, result ExternalCaregiverResult, message string) {
	ctx := r.Context()
	staff, person, err := rs.externalCaregiver(ctx, result.StaffID)
	if err != nil {
		rs.internal(w, r, err)
		return
	}
	response := externalStaffResponse{
		StaffResponse: buildStaffResponse(rs.fieldAccess(ctx), staff, person, true, enrichment{invitationPending: true}),
	}
	if rs.runtime.ExposeSeedInvitationToken(r) {
		response.InvitationToken = result.InvitationToken
	}
	rs.respond(w, r, status, response, message)
}

// externalCaregiver reads the staff member with its external caregiver record marker and person.
func (rs *Resource) externalCaregiver(ctx context.Context, staffID int64) (schoolmembership.Staff, *Person, error) {
	members, err := rs.membership.ListStaff(ctx, schoolmembership.StaffFilter{IDs: []int64{staffID}})
	if err != nil {
		return schoolmembership.Staff{}, nil, err
	}
	if len(members) != 1 {
		return schoolmembership.Staff{}, nil, schoolmembership.ErrStaffNotFound
	}
	person, err := rs.personFor(ctx, members[0])
	if err != nil {
		return schoolmembership.Staff{}, nil, err
	}
	return members[0], person, nil
}
