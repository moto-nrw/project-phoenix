package authorize_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	baseModel "github.com/moto-nrw/project-phoenix/models/base"
)

func systemRole(name string) *authModels.Role {
	return &authModels.Role{Model: baseModel.Model{ID: 1}, Name: name, IsSystem: true}
}

func tenantRole(name string, baseRole *string) *authModels.Role {
	tenantID := int64(7)
	return &authModels.Role{
		Model:    baseModel.Model{ID: 2},
		Name:     name,
		TenantID: &tenantID,
		BaseRole: baseRole,
	}
}

func TestEffectiveBaseRole(t *testing.T) {
	admin := authModels.BaseRoleAdmin
	user := authModels.BaseRoleUser

	tests := []struct {
		name string
		role *authModels.Role
		want string
	}{
		{"nil role", nil, ""},
		{"system admin by name", systemRole("admin"), authModels.BaseRoleAdmin},
		{"system user by name", systemRole("user"), authModels.BaseRoleUser},
		{"system role name is case-insensitive", systemRole("Admin"), authModels.BaseRoleAdmin},
		{"legacy system role stays unknown", systemRole("teacher"), ""},
		{"custom role uses base_role", tenantRole("Sekretariat", &user), authModels.BaseRoleUser},
		{"custom role may be admin tier", tenantRole("Leitung", &admin), authModels.BaseRoleAdmin},
		{"custom role without base_role is unknown", tenantRole("Legacy", nil), ""},
		{"base_role wins over name", tenantRole("admin", &user), authModels.BaseRoleUser},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, authorize.EffectiveBaseRole(tt.role))
		})
	}
}

func TestCanGrantRole(t *testing.T) {
	admin := authModels.BaseRoleAdmin
	user := authModels.BaseRoleUser

	// The escalation this guards: "user" (Betreuer) carries users:create
	// globally, so users:create must not be enough to hand out an admin role.
	betreuer := []string{permissions.UsersCreate, permissions.UsersUpdate}
	adminPerms := []string{permissions.UsersManage}

	tests := []struct {
		name        string
		role        *authModels.Role
		permissions []string
		want        bool
	}{
		{"nil role is never grantable", nil, adminPerms, false},
		{"users:create may not grant the admin role", systemRole("admin"), betreuer, false},
		{"users:manage may grant the admin role", systemRole("admin"), adminPerms, true},
		{"admin wildcard may grant the admin role", systemRole("admin"), []string{permissions.AdminWildcard}, true},
		{"users:create may grant the user role", systemRole("user"), betreuer, true},
		{"no permissions may still grant the user role", systemRole("user"), nil, true},
		{"custom admin-tier role needs users:manage", tenantRole("Leitung", &admin), betreuer, false},
		{"custom user-tier role does not", tenantRole("Sekretariat", &user), betreuer, true},
		{"unknown tier fails closed", tenantRole("Legacy", nil), betreuer, false},
		{"unknown tier is grantable by an admin", tenantRole("Legacy", nil), adminPerms, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, authorize.CanGrantRole(tt.role, tt.permissions))
		})
	}
}
