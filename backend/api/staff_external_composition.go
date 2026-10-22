package api

import (
	"context"
	"net/http"

	chiRender "github.com/go-chi/render"

	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	staffHTTP "github.com/moto-nrw/project-phoenix/modules/schoolmembership/http"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/workflows/externalcaregiver"
)

// externalCaregiverErrorRules map the refusals of the external-caregiver
// workflow (#4573) and of the invitation it issues. The address errors name
// the field the form marks.
var externalCaregiverErrorRules = []apiCommon.ErrorRule{
	{Target: externalcaregiver.ErrEmailRequired, Render: externalFieldCode(apiCommon.ErrorInvalidOnField, apiCommon.CodeStaffExternalEmailRequired)},
	{Target: externalcaregiver.ErrEmailInvalid, Render: externalFieldCode(apiCommon.ErrorInvalidOnField, apiCommon.CodeStaffExternalEmailInvalid)},
	{Target: identityaccess.ErrAccountAlreadyHasTenantAccess, Render: externalFieldCode(apiCommon.ErrorConflictOnField, apiCommon.CodeIdentityAccountAlreadyHasTenantAccess)},
	{Target: identityaccess.ErrEmailAlreadyExists, Render: externalFieldCode(apiCommon.ErrorConflictOnField, apiCommon.CodeIdentityEmailAlreadyExists)},
	{Target: identityaccess.ErrRoleGrantNotPermitted, Render: apiCommon.FixedRenderer(apiCommon.ErrorForbidden, identityaccess.ErrRoleGrantNotPermitted)},
	{Target: externalcaregiver.ErrRoleMissing, Render: externalConflictCode(apiCommon.CodeStaffExternalRoleMissing)},
	{Target: externalcaregiver.ErrNotExternal, Render: externalConflictCode(apiCommon.CodeStaffNotExternal)},
	{Target: externalcaregiver.ErrHasAccount, Render: externalConflictCode(apiCommon.CodeStaffExternalHasAccount)},
	{Target: externalcaregiver.ErrStaffNotFound, Render: apiCommon.ErrorNotFound},
}

func externalFieldCode(render func(error, string, string) chiRender.Renderer, code string) func(error) chiRender.Renderer {
	return func(err error) chiRender.Renderer { return render(err, code, "email") }
}

func externalConflictCode(code string) func(error) chiRender.Renderer {
	return func(err error) chiRender.Renderer { return apiCommon.ErrorConflictWithCode(err, code) }
}

// renderExternalCaregiverFailure renders and observes a refusal of the
// external-caregiver writes; anything unclassified is a 500, which rolls the
// request transaction back.
func renderExternalCaregiverFailure(w http.ResponseWriter, r *http.Request, err error) {
	renderer := apiCommon.RenderWithRules(err, externalCaregiverErrorRules, apiCommon.ErrorInternalServer)
	status := http.StatusInternalServerError
	if response, ok := renderer.(*apiCommon.ErrResponse); ok {
		status = response.HTTPStatusCode
	}
	observability.ObserveSchoolMembershipHTTPResponse(status, "external_caregiver")
	apiCommon.RenderError(w, r, renderer)
}

func externalCaregiverCreate(workflow *externalcaregiver.Workflow) func(context.Context, staffHTTP.ExternalCaregiverInput) (staffHTTP.ExternalCaregiverResult, error) {
	return func(ctx context.Context, input staffHTTP.ExternalCaregiverInput) (staffHTTP.ExternalCaregiverResult, error) {
		result, err := workflow.Record(ctx, externalcaregiver.RecordInput{
			FirstName: input.FirstName, LastName: input.LastName, Email: input.Email, Organization: input.Organization,
			Actor: externalcaregiver.Actor{AccountID: input.ActorAccountID, Permissions: input.ActorPermissions},
		})
		if err != nil {
			return staffHTTP.ExternalCaregiverResult{}, err
		}
		return staffHTTP.ExternalCaregiverResult{StaffID: result.StaffID, InvitationToken: result.InvitationToken}, nil
	}
}

func externalCaregiverInvite(workflow *externalcaregiver.Workflow) func(context.Context, staffHTTP.ExternalInvitationInput) (staffHTTP.ExternalCaregiverResult, error) {
	return func(ctx context.Context, input staffHTTP.ExternalInvitationInput) (staffHTTP.ExternalCaregiverResult, error) {
		result, err := workflow.Invite(ctx, externalcaregiver.InviteInput{
			StaffID: input.StaffID, Email: input.Email,
			Actor: externalcaregiver.Actor{AccountID: input.ActorAccountID, Permissions: input.ActorPermissions},
		})
		if err != nil {
			return staffHTTP.ExternalCaregiverResult{}, err
		}
		return staffHTTP.ExternalCaregiverResult{StaffID: result.StaffID, InvitationToken: result.InvitationToken}, nil
	}
}

// pendingInvitationPersons lists the person IDs the school's open
// invitations name.
func pendingInvitationPersons(invitations identityaccess.SchoolInvitations) func(context.Context) (map[int64]bool, error) {
	return func(ctx context.Context) (map[int64]bool, error) {
		pending, err := invitations.ListPendingSchoolInvitations(ctx)
		if err != nil {
			return nil, err
		}
		persons := make(map[int64]bool, len(pending))
		for _, invitation := range pending {
			if invitation.PersonID != nil {
				persons[*invitation.PersonID] = true
			}
		}
		return persons, nil
	}
}
