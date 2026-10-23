package catererportalhttp

import (
	"errors"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/workflows/catererportal"
)

var (
	errInvalidCredentials = errors.New("invalid email or password")
	errAccountInactive    = errors.New("account is inactive")
)

// errorRules map the caterer flows' errors to their codes. A sign-in that
// fails for any reason of the account answers like a wrong password, so the
// caterer login tells nobody which addresses hold other accounts.
var errorRules = []common.ErrorRule{
	{Target: identityaccess.ErrCatererAlreadySupplies, Render: common.FixedCodedRenderer(common.ErrorConflictWithCode, identityaccess.ErrCatererAlreadySupplies, common.CodeMealsSupplyActive)},
	{Target: catererportal.ErrSchoolNotSupplied, Render: common.FixedCodedRenderer(common.ErrorForbiddenWithCode, catererportal.ErrSchoolNotSupplied, common.CodeMealsNotSupplied)},
	{Target: mealplan.ErrOrderNotFound, Render: common.FixedCodedRenderer(common.ErrorNotFoundWithCode, errors.New(mealplan.ErrOrderNotFound.Detail), common.CodeMealsOrderNotFound)},
	{Target: mealplan.ErrDisabled, Render: common.FixedCodedRenderer(common.ErrorForbiddenWithCode, errors.New(mealplan.ErrDisabled.Detail), common.CodeMealsPlanDisabled)},
	{Target: mealplan.ErrRegistrationDisabled, Render: common.FixedCodedRenderer(common.ErrorForbiddenWithCode, errors.New(mealplan.ErrRegistrationDisabled.Detail), common.CodeMealsRegistrationDisabled)},
	{Target: mealplan.ErrInvalidParticipation, Render: common.FixedRenderer(common.ErrorInvalidRequest, errors.New("the window must be at most six weeks"))},
	{Target: identityaccess.ErrCatererAddressTaken, Render: common.FixedCodedRenderer(common.ErrorConflictWithCode, identityaccess.ErrCatererAddressTaken, common.CodeIdentityCatererAddressTaken)},
	{Target: identityaccess.ErrCatererWithoutSchool, Render: common.FixedCodedRenderer(common.ErrorForbiddenWithCode, identityaccess.ErrCatererWithoutSchool, common.CodeIdentityCatererWithoutSchool)},
	{Target: identityaccess.ErrInvalidCatererInvitation, Render: common.FixedRenderer(common.ErrorInvalidRequest, identityaccess.ErrInvalidCatererInvitation)},
	{Target: identityaccess.ErrInvitationNotFound, Render: common.FixedCodedRenderer(common.ErrorNotFoundWithCode, identityaccess.ErrInvitationNotFound, common.CodeIdentityInvitationNotFound)},
	{Target: identityaccess.ErrInvitationExpired, Render: common.FixedCodedRenderer(common.ErrorGoneWithCode, identityaccess.ErrInvitationExpired, common.CodeIdentityInvitationExpired)},
	{Target: identityaccess.ErrInvitationUsed, Render: common.FixedCodedRenderer(common.ErrorGoneWithCode, identityaccess.ErrInvitationUsed, common.CodeIdentityInvitationExpired)},
	{Target: identityaccess.ErrInvitationRevoked, Render: common.FixedCodedRenderer(common.ErrorGoneWithCode, identityaccess.ErrInvitationRevoked, common.CodeIdentityInvitationExpired)},
	{Target: identityaccess.ErrInvitationTenantDeleted, Render: common.FixedCodedRenderer(common.ErrorNotFoundWithCode, identityaccess.ErrInvitationNotFound, common.CodeIdentityInvitationNotFound)},
	{Target: identityaccess.ErrInvitationPasswordMismatch, Render: func(err error) render.Renderer {
		return common.ErrorInvalidOnField(identityaccess.ErrInvitationPasswordMismatch, common.CodeIdentityPasswordMismatch, "confirm_password")
	}},
	{Target: identityaccess.ErrPasswordTooWeak, Render: func(err error) render.Renderer {
		return common.ErrorInvalidOnField(identityaccess.ErrPasswordTooWeak, common.CodeIdentityPasswordTooWeak, "password")
	}},
	{Target: identityaccess.ErrAccountInactive, Render: common.FixedCodedRenderer(common.ErrorUnauthorizedWithCode, errAccountInactive, common.CodeIdentitySessionAccountInactive)},
	{Target: identityaccess.ErrInvalidCredentials, Render: common.FixedCodedRenderer(common.ErrorUnauthorizedWithCode, errInvalidCredentials, common.CodeIdentityInvalidCredentials)},
	{Target: identityaccess.ErrAccountNotFound, Render: common.FixedCodedRenderer(common.ErrorUnauthorizedWithCode, errInvalidCredentials, common.CodeIdentityInvalidCredentials)},
	{Target: identityaccess.ErrAccountNotCaterer, Render: common.FixedCodedRenderer(common.ErrorUnauthorizedWithCode, errInvalidCredentials, common.CodeIdentityInvalidCredentials)},
}
