package api

import (
	"context"
	"net/http"

	chiRender "github.com/go-chi/render"

	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	schoolMembershipModule "github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	staffHTTP "github.com/moto-nrw/project-phoenix/modules/schoolmembership/http"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/externalcaregiver"
)

// bindExternalCaregivers builds the external-caregiver workflow (#4573) as a
// local value of the staff mount and binds it into the School Membership
// HTTP runtime. The owners' public capabilities satisfy the workflow's
// ports; its savepoint runs inside the request's tenant transaction. appEnv
// gates the seed-only invitation token.
func bindExternalCaregivers(
	runtime *staffHTTP.Runtime, module schoolMembershipModule.Capability, people peopleModule.Capability,
	identity identityaccess.Owner, appEnv string,
) error {
	externals, err := externalcaregiver.New(people, module, identity, tenant.WithSavepoint)
	if err != nil {
		return err
	}
	runtime.CreateExternalCaregiver = externalCaregiverCreate(externals)
	runtime.InviteExternalCaregiver = externalCaregiverInvite(externals)
	runtime.ExternalFailure = renderExternalCaregiverFailure
	runtime.ExposeSeedInvitationToken = func(r *http.Request) bool {
		return apiCommon.ShouldExposeSeedInvitationToken(r, appEnv)
	}
	runtime.PendingInvitationPersons = pendingInvitationPersons(identity)
	return nil
}

// externalCaregiverErrorRules map the refusals of the external-caregiver
// workflow (#4573) and of the invitation it issues. The access and address
// errors name the field the form marks.
var externalCaregiverErrorRules = []apiCommon.ErrorRule{
	{Target: externalcaregiver.ErrAccessInvalid, Render: externalFieldCode(apiCommon.ErrorInvalidOnField, apiCommon.CodeStaffExternalAccessInvalid, "access")},
	{Target: externalcaregiver.ErrEmailRequired, Render: externalFieldCode(apiCommon.ErrorInvalidOnField, apiCommon.CodeStaffExternalEmailRequired, "email")},
	{Target: externalcaregiver.ErrEmailInvalid, Render: externalFieldCode(apiCommon.ErrorInvalidOnField, apiCommon.CodeStaffExternalEmailInvalid, "email")},
	{Target: identityaccess.ErrAccountAlreadyHasTenantAccess, Render: externalFieldCode(apiCommon.ErrorConflictOnField, apiCommon.CodeIdentityAccountAlreadyHasTenantAccess, "email")},
	{Target: identityaccess.ErrEmailAlreadyExists, Render: externalFieldCode(apiCommon.ErrorConflictOnField, apiCommon.CodeIdentityEmailAlreadyExists, "email")},
	{Target: externalcaregiver.ErrEmailInvited, Render: externalFieldCode(apiCommon.ErrorConflictOnField, apiCommon.CodeStaffExternalEmailInvited, "email")},
	{Target: identityaccess.ErrRoleGrantNotPermitted, Render: apiCommon.FixedRenderer(apiCommon.ErrorForbidden, identityaccess.ErrRoleGrantNotPermitted)},
	{Target: externalcaregiver.ErrNotExternal, Render: externalConflictCode(apiCommon.CodeStaffNotExternal)},
	{Target: externalcaregiver.ErrHasAccount, Render: externalConflictCode(apiCommon.CodeStaffExternalHasAccount)},
	{Target: externalcaregiver.ErrStaffNotFound, Render: apiCommon.ErrorNotFound},
}

func externalFieldCode(render func(error, string, string) chiRender.Renderer, code, field string) func(error) chiRender.Renderer {
	return func(err error) chiRender.Renderer { return render(err, code, field) }
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
			Access: externalcaregiver.Access(input.Access),
			Actor:  externalcaregiver.Actor{AccountID: input.ActorAccountID, Permissions: input.ActorPermissions},
		})
		if err != nil {
			return staffHTTP.ExternalCaregiverResult{}, err
		}
		return staffHTTP.ExternalCaregiverResult{StaffID: result.StaffID, Invited: result.Invited, InvitationToken: result.InvitationToken}, nil
	}
}

func externalCaregiverInvite(workflow *externalcaregiver.Workflow) func(context.Context, staffHTTP.ExternalInvitationInput) (staffHTTP.ExternalCaregiverResult, error) {
	return func(ctx context.Context, input staffHTTP.ExternalInvitationInput) (staffHTTP.ExternalCaregiverResult, error) {
		result, err := workflow.Invite(ctx, externalcaregiver.InviteInput{
			StaffID: input.StaffID, Email: input.Email, Access: externalcaregiver.Access(input.Access),
			Actor: externalcaregiver.Actor{AccountID: input.ActorAccountID, Permissions: input.ActorPermissions},
		})
		if err != nil {
			return staffHTTP.ExternalCaregiverResult{}, err
		}
		return staffHTTP.ExternalCaregiverResult{StaffID: result.StaffID, Invited: result.Invited, InvitationToken: result.InvitationToken}, nil
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
		byPerson := identityaccess.PendingInvitationsByPerson(pending)
		persons := make(map[int64]bool, len(byPerson))
		for personID := range byPerson {
			persons[personID] = true
		}
		return persons, nil
	}
}
