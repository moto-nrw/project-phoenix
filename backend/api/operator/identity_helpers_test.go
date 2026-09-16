package operator_test

import (
	"context"

	"github.com/moto-nrw/project-phoenix/api/operator"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityoperator "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"
)

// newIdentityResource builds the Identity & Access operator routes the
// operator router mounts (#3252), answering in the operator error format.
func newIdentityResource(identity identityoperator.Capability) *identityoperator.Resource {
	return identityoperator.NewResource(identity, operator.IdentityResponses())
}

func identityOperatorOf(op *platform.Operator) *identityaccess.Operator {
	if op == nil {
		return nil
	}
	return &identityaccess.Operator{ID: op.ID, Email: op.Email, DisplayName: op.DisplayName, Active: op.Active}
}

// The school-access half of the capability is exercised by the account
// access tests; the auth and profile cases never reach it.

func (m *mockOperatorAuthService) ListAccountTenantAccess(context.Context, int64) ([]identityaccess.AccountTenantAccess, error) {
	return nil, nil
}

func (m *mockOperatorAuthService) ListAssignableSchoolRoles(context.Context, int64) ([]identityaccess.AccountTenantRole, error) {
	return nil, nil
}

func (m *mockOperatorAuthService) GrantAccountTenantAccess(context.Context, identityaccess.GrantAccountTenantAccess) ([]identityaccess.AccountTenantAccess, error) {
	return nil, nil
}

func (m *mockOperatorAuthService) UpdateAccountTenantRole(context.Context, int64, int64, int64, int64, string) ([]identityaccess.AccountTenantAccess, error) {
	return nil, nil
}

func (m *mockOperatorAuthService) RevokeAccountTenantAccess(context.Context, int64, int64, int64, string) ([]identityaccess.AccountTenantAccess, error) {
	return nil, nil
}
