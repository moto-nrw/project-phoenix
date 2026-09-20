package compose

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/dataimport"
	importapi "github.com/moto-nrw/project-phoenix/modules/dataimport/inbound"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// HTTPRuntime binds infrastructure and owner queries outside the handlers.
func HTTPRuntime(db *bun.DB, people peopledirectory.Capability, membership schoolmembership.Capability,
	opening dataimport.OpeningBalanceFactory,
) importapi.Runtime {
	staffForAccount := func(ctx context.Context, accountID int64) (int64, error) {
		person, err := people.FindPersonByAccount(ctx, accountID)
		if err != nil {
			return 0, fmt.Errorf("find person for account %d: %w", accountID, err)
		}
		staff, err := membership.FindStaffByPerson(ctx, person.ID)
		if err != nil {
			return 0, fmt.Errorf("find staff for person %d: %w", person.ID, err)
		}
		return staff.ID, nil
	}
	runtime := importapi.Runtime{
		Middleware: []importapi.Middleware{
			jwtauth.Verifier(jwt.MustNewTokenAuth().JwtAuth), jwt.Authenticator,
			common.ReadOnlyPreviewMiddleware, common.TenantScopeMiddleware,
			common.SecurityPrincipalMiddleware, common.TenantOperationMiddleware,
		},
		RequireAnyPermission: common.RequiresAnyPermission,
		TemplateTransaction:  common.TenantTxMiddleware,
		WithinTenant: func(ctx context.Context, fn func(context.Context) error) error {
			return tenant.WithTenantTx(ctx, db, tenant.FromContext(ctx), func(ctx context.Context, _ bun.Tx) error { return fn(ctx) })
		},
		TenantID:    tenant.FromContext,
		AccountID:   accountID,
		Permissions: func(ctx context.Context) []string { return jwt.ClaimsFromCtx(ctx).Permissions },
		StaffID: func(ctx context.Context) (int64, error) {
			id, err := accountID(ctx)
			if err != nil {
				return 0, err
			}
			return staffForAccount(ctx, id)
		},
		OpeningDecider: staffForAccount,
		Success:        common.Respond,
		Failure:        renderFailure,
	}
	if opening != nil {
		runtime.ValidateOpeningDate = opening.ValidateDate
		runtime.OpeningImport = opening.ForUpload
	}
	return runtime
}

func accountID(ctx context.Context) (int64, error) {
	claims, ok := ctx.Value(jwt.CtxClaims).(jwt.AppClaims)
	if !ok {
		return 0, fmt.Errorf("no claims in context")
	}
	return int64(claims.ID), nil
}

func renderFailure(w http.ResponseWriter, r *http.Request, failure importapi.Failure) {
	var response render.Renderer
	switch {
	case failure.Code != "":
		response = &common.ErrResponse{Err: failure.Cause, HTTPStatusCode: failure.Status,
			Status: "error", ErrorText: failure.Message, Code: failure.Code,
			Details: map[string]any{"result": failure.Result}}
	case failure.Status == http.StatusBadRequest:
		response = common.ErrorInvalidRequest(failure.Cause)
	case failure.Status == http.StatusUnauthorized:
		response = common.ErrorUnauthorized(failure.Cause)
	case failure.Status == http.StatusForbidden:
		response = common.ErrorForbiddenMessage(failure.Message)
	case failure.Message != "":
		response = common.ErrorInternalServerWrap(failure.Message, failure.Cause)
	default:
		response = common.ErrorInternalServer(failure.Cause)
	}
	common.RenderError(w, r, response)
}
