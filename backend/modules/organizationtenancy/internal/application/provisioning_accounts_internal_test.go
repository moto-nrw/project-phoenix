package application

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvisioningInviteSchoolAdmin(t *testing.T) {
	t.Parallel()

	input := organizationtenancy.SchoolAdminInvitationInput{
		Email: "  Leitung@Burbach.TEST ", FirstName: strPtr("Lea"), LastName: strPtr("Leitung"),
		Position: strPtr("Schulleitung"), CaregiverEnabled: true,
	}

	t.Run("invites the school admin with the admin system role", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDeviceSchools(h)

		invitation, err := h.svc.InviteSchoolAdmin(context.Background(), 10, testOperatorID, operatorIP, input)

		require.NoError(t, err)
		assert.Equal(t, []domain.SchoolAdminInvitationRequest{{
			TenantID: 10, RoleID: 1, Email: "leitung@burbach.test",
			FirstName: input.FirstName, LastName: input.LastName, Position: input.Position, CaregiverEnabled: true,
		}}, h.identity.invitations)
		assert.Equal(t, &organizationtenancy.SchoolAdminInvitation{
			ID: 900, Email: "leitung@burbach.test", RoleID: 1, RoleName: "admin", Token: "invite-token",
			ExpiresAt: fixedTime.Add(48 * time.Hour), FirstName: input.FirstName, LastName: input.LastName,
			Position: input.Position, CaregiverEnabled: true,
		}, invitation)
		entry := h.onlyAudit(t, domain.AuditActionCreate, domain.AuditResourceInvitation, 900)
		assert.Equal(t, map[string]any{
			"schoolID": float64(10), "email": "leitung@burbach.test", "roleID": float64(1),
		}, auditChanges(t, entry))
	})

	t.Run("a tenant role named admin is not the system role", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDeviceSchools(h)
		h.identity.roles = []domain.Role{{ID: 50, Name: "admin", IsSystem: true, TenantID: int64Ptr(10)}}

		invitation, err := h.svc.InviteSchoolAdmin(context.Background(), 10, testOperatorID, operatorIP, input)

		assert.Nil(t, invitation)
		var invalid *organizationtenancy.InvalidProvisioningDataError
		require.ErrorAs(t, err, &invalid)
		assert.Contains(t, invalid.Error(), "admin role not found")
		assert.Empty(t, h.identity.invitations)
	})

	t.Run("audit failure does not fail the invitation", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDeviceSchools(h)
		h.audit.err = errBoom

		invitation, err := h.svc.InviteSchoolAdmin(context.Background(), 10, testOperatorID, operatorIP, input)

		require.NoError(t, err)
		require.NotNil(t, invitation)
	})

	for name, tc := range map[string]struct {
		seed  func(*provisioningHarness)
		check func(*testing.T, error)
	}{
		"missing school": {
			seed: func(h *provisioningHarness) { delete(h.engine.schools, 10) },
			check: func(t *testing.T, err error) {
				var notFound *organizationtenancy.SchoolNotFoundError
				require.ErrorAs(t, err, &notFound)
				assert.Equal(t, int64(10), notFound.SchoolID)
			},
		},
		"deleted school": {
			seed: func(h *provisioningHarness) { h.engine.schools[10].DeletedAt = deletedNow() },
			check: func(t *testing.T, err error) {
				var deleted *organizationtenancy.SchoolAlreadyDeletedError
				require.ErrorAs(t, err, &deleted)
			},
		},
		"inactive school": {
			seed: func(h *provisioningHarness) { h.engine.schools[10].Active = false },
			check: func(t *testing.T, err error) {
				var invalid *organizationtenancy.InvalidProvisioningDataError
				require.ErrorAs(t, err, &invalid)
				assert.Contains(t, invalid.Error(), "school is inactive")
			},
		},
		"missing admin role": {
			seed: func(h *provisioningHarness) { h.identity.roles = nil },
			check: func(t *testing.T, err error) {
				var invalid *organizationtenancy.InvalidProvisioningDataError
				require.ErrorAs(t, err, &invalid)
				assert.Contains(t, invalid.Error(), "admin role not found")
			},
		},
		"role lookup failure": {
			seed:  func(h *provisioningHarness) { h.identity.fail["FindSystemRole"] = errBoom },
			check: func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
		"invitation failure": {
			seed:  func(h *provisioningHarness) { h.identity.fail["InviteSchoolAdmin"] = errBoom },
			check: func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			seedDeviceSchools(h)
			tc.seed(h)

			invitation, err := h.svc.InviteSchoolAdmin(context.Background(), 10, testOperatorID, operatorIP, input)

			assert.Nil(t, invitation)
			tc.check(t, err)
			assert.Empty(t, h.identity.invitations)
			assert.Empty(t, h.audit.entries)
		})
	}
}

func accountInput(roleID *int64) organizationtenancy.SchoolAccountInput {
	return organizationtenancy.SchoolAccountInput{
		Email: "jane@example.test", Password: "SecureP@ss1", FirstName: " Jane ", LastName: "DOE",
		RoleID: roleID,
	}
}

func TestProvisioningCreateSchoolAccount(t *testing.T) {
	t.Parallel()

	t.Run("creates the account and its school identity", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDeviceSchools(h)
		input := accountInput(int64Ptr(2))
		input.Position = "Klassenlehrerin"

		account, err := h.svc.CreateSchoolAccount(context.Background(), 10, testOperatorID, operatorIP, input)

		require.NoError(t, err)
		require.NotNil(t, account)
		assert.Equal(t, int64(100), account.ID)
		assert.Equal(t, "jane@example.test", account.Email)
		assert.Equal(t, []domain.SchoolAccountRegistration{{
			TenantID: 10, Email: "jane@example.test", Username: "jane_doe_abc123", Password: "SecureP@ss1", RoleID: 2,
		}}, h.identity.registrations)
		require.Len(t, h.identity.identities, 1)
		assert.Equal(t, domain.SchoolIdentityRequest{
			AccountID: 100, TenantID: 10, Role: h.identity.roles[1],
			FirstName: " Jane ", LastName: "DOE", Position: "Klassenlehrerin",
		}, h.identity.identities[0])
		assert.Empty(t, h.identity.assigned)
		entry := h.onlyAudit(t, domain.AuditActionCreate, domain.AuditResourceAccount, 100)
		assert.Equal(t, map[string]any{"schoolID": float64(10), "email": "jane@example.test"}, auditChanges(t, entry))
	})

	t.Run("defaults to the admin system role", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDeviceSchools(h)

		_, err := h.svc.CreateSchoolAccount(context.Background(), 10, testOperatorID, operatorIP, accountInput(nil))

		require.NoError(t, err)
		require.Len(t, h.identity.registrations, 1)
		assert.Equal(t, int64(1), h.identity.registrations[0].RoleID)
		assert.Equal(t, "admin", h.identity.identities[0].Role.Name)
		assert.NotContains(t, h.identity.calls, "FindRole")
	})

	t.Run("a lehrkraft without caregiver capability is allowed", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDeviceSchools(h)

		_, err := h.svc.CreateSchoolAccount(context.Background(), 10, testOperatorID, operatorIP, accountInput(int64Ptr(5)))

		require.NoError(t, err)
		assert.Equal(t, int64(5), h.identity.registrations[0].RoleID)
		assert.Empty(t, h.identity.assigned)
	})

	t.Run("caregiver upgrade hands out the user role", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDeviceSchools(h)
		input := accountInput(int64Ptr(6))
		input.CaregiverEnabled = true

		_, err := h.svc.CreateSchoolAccount(context.Background(), 10, testOperatorID, operatorIP, input)

		require.NoError(t, err)
		assert.True(t, h.identity.identities[0].CaregiverUpgrade)
		assert.Equal(t, [][3]int64{{10, 100, 2}}, h.identity.assigned)
	})

	t.Run("a role with caregiver permissions needs no extra role", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDeviceSchools(h)
		input := accountInput(nil)
		input.CaregiverEnabled = true

		_, err := h.svc.CreateSchoolAccount(context.Background(), 10, testOperatorID, operatorIP, input)

		require.NoError(t, err)
		assert.True(t, h.identity.identities[0].CaregiverUpgrade)
		assert.Empty(t, h.identity.assigned)
	})

	t.Run("audit failure does not fail the creation", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDeviceSchools(h)
		h.audit.err = errBoom

		account, err := h.svc.CreateSchoolAccount(context.Background(), 10, testOperatorID, operatorIP, accountInput(nil))

		require.NoError(t, err)
		require.NotNil(t, account)
	})

	caregiver := func(input organizationtenancy.SchoolAccountInput) organizationtenancy.SchoolAccountInput {
		input.CaregiverEnabled = true
		return input
	}
	for name, tc := range map[string]struct {
		seed    func(*provisioningHarness)
		input   organizationtenancy.SchoolAccountInput
		message string
	}{
		"unknown role": {input: accountInput(int64Ptr(99)), message: "role with ID 99 not found"},
		"non-system role": {
			seed: func(h *provisioningHarness) {
				h.identity.roles = append(h.identity.roles, domain.Role{ID: 60, Name: "custom", TenantID: int64Ptr(10)})
			},
			input:   accountInput(int64Ptr(60)),
			message: "only system roles are allowed",
		},
		"guardian role": {input: accountInput(int64Ptr(3)), message: "guardian invitation flow"},
		"legacy teacher role": {
			seed:    func(h *provisioningHarness) { h.identity.roles[3].Name = "Teacher" },
			input:   accountInput(int64Ptr(4)),
			message: "legacy teacher role is no longer assignable",
		},
		"lehrkraft with caregiver capability": {
			input:   caregiver(accountInput(int64Ptr(5))),
			message: "lehrkraft role cannot be combined with caregiver capability",
		},
		"missing admin role": {
			seed:    func(h *provisioningHarness) { h.identity.roles = h.identity.roles[1:] },
			input:   accountInput(nil),
			message: "admin role not found",
		},
		"inactive school": {
			seed:    func(h *provisioningHarness) { h.engine.schools[10].Active = false },
			input:   accountInput(nil),
			message: "school is inactive",
		},
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			seedDeviceSchools(h)
			if tc.seed != nil {
				tc.seed(h)
			}

			account, err := h.svc.CreateSchoolAccount(context.Background(), 10, testOperatorID, operatorIP, tc.input)

			assert.Nil(t, account)
			var invalid *organizationtenancy.InvalidProvisioningDataError
			require.ErrorAs(t, err, &invalid)
			assert.Contains(t, invalid.Error(), tc.message)
			assert.Empty(t, h.identity.registrations, "no account for a rejected request")
			assert.Empty(t, h.audit.entries)
		})
	}

	for name, tc := range map[string]struct {
		seed  func(*provisioningHarness)
		check func(*testing.T, error)
	}{
		"missing school": {
			seed: func(h *provisioningHarness) { delete(h.engine.schools, 10) },
			check: func(t *testing.T, err error) {
				var notFound *organizationtenancy.SchoolNotFoundError
				require.ErrorAs(t, err, &notFound)
				assert.Equal(t, int64(10), notFound.SchoolID)
			},
		},
		"deleted school": {
			seed: func(h *provisioningHarness) { h.engine.schools[10].DeletedAt = deletedNow() },
			check: func(t *testing.T, err error) {
				var deleted *organizationtenancy.SchoolAlreadyDeletedError
				require.ErrorAs(t, err, &deleted)
				assert.Equal(t, int64(10), deleted.SchoolID)
			},
		},
		"role lookup failure": {
			seed: func(h *provisioningHarness) { h.identity.fail["FindRole"] = errBoom },
			check: func(t *testing.T, err error) {
				require.ErrorIs(t, err, errBoom)
				assert.Contains(t, err.Error(), "lookup role")
			},
		},
		"registration failure": {
			seed:  func(h *provisioningHarness) { h.identity.fail["RegisterSchoolAccount"] = errBoom },
			check: func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			seedDeviceSchools(h)
			tc.seed(h)

			account, err := h.svc.CreateSchoolAccount(context.Background(), 10, testOperatorID, operatorIP, accountInput(int64Ptr(2)))

			assert.Nil(t, account)
			tc.check(t, err)
			assert.Empty(t, h.identity.identities)
			assert.Empty(t, h.audit.entries)
		})
	}

	for name, tc := range map[string]struct {
		seed  func(*provisioningHarness)
		input organizationtenancy.SchoolAccountInput
		check func(*testing.T, error)
	}{
		"invalid identity input": {
			seed: func(h *provisioningHarness) {
				h.identity.fail["EnsureSchoolIdentity"] = fmt.Errorf("names required: %w", domain.ErrInvalidSchoolIdentity)
			},
			input: accountInput(int64Ptr(2)),
			check: func(t *testing.T, err error) {
				var invalid *organizationtenancy.InvalidProvisioningDataError
				require.ErrorAs(t, err, &invalid)
				require.ErrorIs(t, err, domain.ErrInvalidSchoolIdentity)
			},
		},
		"identity failure": {
			seed:  func(h *provisioningHarness) { h.identity.fail["EnsureSchoolIdentity"] = errBoom },
			input: accountInput(int64Ptr(2)),
			check: func(t *testing.T, err error) {
				require.ErrorIs(t, err, errBoom)
				var invalid *organizationtenancy.InvalidProvisioningDataError
				assert.NotErrorAs(t, err, &invalid)
			},
		},
		"missing user role for the caregiver upgrade": {
			seed:  func(h *provisioningHarness) { h.identity.roles = append(h.identity.roles[:1], h.identity.roles[2:]...) },
			input: caregiver(accountInput(int64Ptr(6))),
			check: func(t *testing.T, err error) {
				require.ErrorContains(t, err, "assign caregiver role: user role not found")
			},
		},
		"user role lookup failure": {
			seed: func(h *provisioningHarness) {
				// The account role is resolved by ID, so only the user role
				// lookup fails.
				h.identity.fail["FindSystemRole"] = errBoom
			},
			input: caregiver(accountInput(int64Ptr(6))),
			check: func(t *testing.T, err error) {
				require.ErrorIs(t, err, errBoom)
				assert.Contains(t, err.Error(), "assign caregiver role")
			},
		},
		"role assignment failure": {
			seed:  func(h *provisioningHarness) { h.identity.fail["AssignRole"] = errBoom },
			input: caregiver(accountInput(int64Ptr(6))),
			check: func(t *testing.T, err error) {
				require.ErrorIs(t, err, errBoom)
				assert.Contains(t, err.Error(), "assign caregiver role")
			},
		},
	} {
		t.Run(name+" fails after registration", func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			seedDeviceSchools(h)
			tc.seed(h)

			account, err := h.svc.CreateSchoolAccount(context.Background(), 10, testOperatorID, operatorIP, tc.input)

			assert.Nil(t, account)
			tc.check(t, err)
			assert.Len(t, h.identity.registrations, 1)
			require.Error(t, h.tx.lastErr, "the registered account must roll back")
			assert.Empty(t, h.audit.entries)
		})
	}
}

func TestProvisioningListSystemRoles(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.identity.roles = []domain.Role{
		{ID: 1, Name: "admin", IsSystem: true},
		{ID: 9, Name: "custom", TenantID: int64Ptr(10)},
		{ID: 2, Name: "user", IsSystem: true},
	}

	roles, err := h.svc.ListSystemRoles(context.Background())

	require.NoError(t, err)
	assert.Equal(t, []organizationtenancy.SystemRole{
		{ID: 1, Name: "admin", IsSystem: true},
		{ID: 2, Name: "user", IsSystem: true},
	}, roles)

	h.identity.fail["ListSystemRoles"] = errBoom
	roles, err = h.svc.ListSystemRoles(context.Background())
	assert.Nil(t, roles)
	require.ErrorIs(t, err, errBoom)
}

func schoolAccount(id int64, email string) domain.SchoolAccount {
	return domain.SchoolAccount{
		AccountID: id, Email: email, Active: true, FirstName: "Jane", LastName: "Doe",
		RoleName: "admin", PedagogicRole: "Leitung", Status: "active",
		HasAdminRole: true, HasCaregiverProfile: true, IsActiveCaregiver: true,
	}
}

func TestProvisioningListSchoolAccounts(t *testing.T) {
	t.Parallel()

	t.Run("maps the school's accounts", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDeviceSchools(h)
		h.identity.schoolAccts = []domain.SchoolAccount{schoolAccount(1, "a@example.test"), schoolAccount(2, "b@example.test")}

		accounts, err := h.svc.ListSchoolAccounts(context.Background(), 10)

		require.NoError(t, err)
		require.Len(t, accounts, 2)
		assert.Equal(t, organizationtenancy.SchoolAccount(h.identity.schoolAccts[0]), accounts[0])
		assert.Equal(t, "b@example.test", accounts[1].Email)
	})

	t.Run("no accounts stays nil", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDeviceSchools(h)

		accounts, err := h.svc.ListSchoolAccounts(context.Background(), 10)

		require.NoError(t, err)
		assert.Nil(t, accounts)
	})

	for name, tc := range map[string]struct {
		seed  func(*provisioningHarness)
		check func(*testing.T, error)
	}{
		"missing school": {
			seed: func(h *provisioningHarness) { delete(h.engine.schools, 10) },
			check: func(t *testing.T, err error) {
				var notFound *organizationtenancy.SchoolNotFoundError
				require.ErrorAs(t, err, &notFound)
			},
		},
		"deleted school": {
			seed: func(h *provisioningHarness) { h.engine.schools[10].DeletedAt = deletedNow() },
			check: func(t *testing.T, err error) {
				var deleted *organizationtenancy.SchoolAlreadyDeletedError
				require.ErrorAs(t, err, &deleted)
			},
		},
		"listing failure": {
			seed:  func(h *provisioningHarness) { h.identity.fail["ListSchoolAccounts"] = errBoom },
			check: func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			seedDeviceSchools(h)
			tc.seed(h)

			accounts, err := h.svc.ListSchoolAccounts(context.Background(), 10)

			assert.Nil(t, accounts)
			tc.check(t, err)
		})
	}
}

func TestProvisioningListOrganizationAndAllAccounts(t *testing.T) {
	t.Parallel()

	rows := []domain.OrganizationAccount{
		{SchoolAccount: schoolAccount(1, "a@example.test"), SchoolID: 10, SchoolName: "Burbach"},
		{SchoolAccount: schoolAccount(2, "b@example.test"), SchoolID: 20, SchoolName: "Walbach"},
	}
	want := []organizationtenancy.OrganizationAccount{
		{SchoolAccount: organizationtenancy.SchoolAccount(rows[0].SchoolAccount), SchoolID: 10, SchoolName: "Burbach"},
		{SchoolAccount: organizationtenancy.SchoolAccount(rows[1].SchoolAccount), SchoolID: 20, SchoolName: "Walbach"},
	}

	t.Run("organisation accounts", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDeviceSchools(h)
		h.identity.orgAccts = rows

		accounts, err := h.svc.ListOrganizationAccounts(context.Background(), 1)

		require.NoError(t, err)
		assert.Equal(t, want, accounts)
		assert.Equal(t, []int64{1}, h.identity.orgAcctsFor)
	})

	t.Run("missing organisation", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		accounts, err := h.svc.ListOrganizationAccounts(context.Background(), 99)

		assert.Nil(t, accounts)
		var notFound *organizationtenancy.OrganizationNotFoundError
		require.ErrorAs(t, err, &notFound)
		assert.Equal(t, int64(99), notFound.OrganizationID)
		assert.Empty(t, h.identity.orgAcctsFor)
	})

	t.Run("organisation listing failure", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedDeviceSchools(h)
		h.identity.fail["ListOrganizationAccounts"] = errBoom

		accounts, err := h.svc.ListOrganizationAccounts(context.Background(), 1)

		assert.Nil(t, accounts)
		require.ErrorIs(t, err, errBoom)
	})

	t.Run("all accounts", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.identity.orgAccts = rows

		accounts, err := h.svc.ListAllAccounts(context.Background())

		require.NoError(t, err)
		assert.Equal(t, want, accounts)
	})

	t.Run("no accounts stays nil", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		accounts, err := h.svc.ListAllAccounts(context.Background())

		require.NoError(t, err)
		assert.Nil(t, accounts)
	})

	t.Run("all accounts failure", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.identity.fail["ListAllAccounts"] = errBoom

		accounts, err := h.svc.ListAllAccounts(context.Background())

		assert.Nil(t, accounts)
		require.ErrorIs(t, err, errBoom)
	})
}
