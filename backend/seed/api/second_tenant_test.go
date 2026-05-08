package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/integration/phoenixapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// SecondTenantOptions.validate
// =============================================================================

func TestSecondTenantOptions_Validate_Required(t *testing.T) {
	cases := []struct {
		name    string
		opts    SecondTenantOptions
		wantErr string
	}{
		{
			name:    "missing slug",
			opts:    SecondTenantOptions{Name: "x", AdminEmail: "a@b", AdminPassword: "p", LinkEmail: "l@b"},
			wantErr: "Slug",
		},
		{
			name:    "missing name",
			opts:    SecondTenantOptions{Slug: "x", AdminEmail: "a@b", AdminPassword: "p", LinkEmail: "l@b"},
			wantErr: "Name",
		},
		{
			name:    "missing admin email",
			opts:    SecondTenantOptions{Slug: "x", Name: "n", AdminPassword: "p", LinkEmail: "l@b"},
			wantErr: "AdminEmail",
		},
		{
			name:    "missing admin password",
			opts:    SecondTenantOptions{Slug: "x", Name: "n", AdminEmail: "a@b", LinkEmail: "l@b"},
			wantErr: "AdminPassword",
		},
		{
			name:    "missing link email",
			opts:    SecondTenantOptions{Slug: "x", Name: "n", AdminEmail: "a@b", AdminPassword: "p"},
			wantErr: "LinkEmail",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.opts.validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestSecondTenantOptions_Validate_OK(t *testing.T) {
	opts := SecondTenantOptions{
		Slug:          "second-school",
		Name:          "Demo School 2",
		AdminEmail:    "admin-b@e2e.local",
		AdminPassword: "Test1234%",
		LinkEmail:     "demo1@mail.de",
	}
	require.NoError(t, opts.validate())
}

// =============================================================================
// isConflict
// =============================================================================

func TestIsConflict(t *testing.T) {
	t.Run("APIError 409 is conflict", func(t *testing.T) {
		err := &phoenixapi.APIError{StatusCode: http.StatusConflict}
		assert.True(t, isConflict(err))
	})
	t.Run("APIError 500 is NOT conflict", func(t *testing.T) {
		err := &phoenixapi.APIError{StatusCode: http.StatusInternalServerError}
		assert.False(t, isConflict(err))
	})
	t.Run("APIError 400 is NOT conflict", func(t *testing.T) {
		err := &phoenixapi.APIError{StatusCode: http.StatusBadRequest}
		assert.False(t, isConflict(err))
	})
	t.Run("APIError 401 is NOT conflict", func(t *testing.T) {
		err := &phoenixapi.APIError{StatusCode: http.StatusUnauthorized}
		assert.False(t, isConflict(err))
	})
	t.Run("plain error is NOT conflict", func(t *testing.T) {
		err := fmt.Errorf("network timeout")
		assert.False(t, isConflict(err))
	})
	t.Run("nil is NOT conflict", func(t *testing.T) {
		assert.False(t, isConflict(nil))
	})
}

// =============================================================================
// secondTenantStep.Run — happy path and error paths
// =============================================================================

// secondTenantMockOpts captures the per-test response shaping used by
// secondTenantMock. Each `*Status` field, when non-zero, is the status
// the mock returns for that endpoint instead of 200/201; lets a test
// inject a 409 (idempotent) or 500 (hard fail) at exactly one step.
type secondTenantMockOpts struct {
	createSchoolStatus int
	inviteStatus       int
	acceptStatus       int
	rolesStatus        int
	linkStatus         int
	// schoolListSlug controls what slug the /operator/schools listing
	// endpoint returns when a 409 falls through to lookup.
	schoolListSlug string
	// verifyLoginStatus, when non-zero, is the status returned by
	// /auth/login during the post-link read-back verification.
	// Used to simulate a contract-drift where the link account can
	// no longer authenticate.
	verifyLoginStatus int
	// verifyTenantsStatus, when non-zero, is the status returned by
	// GET /auth/account/tenants during verification.
	verifyTenantsStatus int
	// verifyTenantSlugs is the list of slugs returned by
	// /auth/account/tenants during verification. nil = default both
	// (demo-school + second-school). An explicit slice (including
	// empty) is used as-is so tests can simulate a missing mapping.
	verifyTenantSlugs *[]string
}

func secondTenantMock(t *testing.T, opts secondTenantMockOpts) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)

		case "/operator/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"access_token": "op-token"},
			})

		case "/auth/login":
			// Two distinct logins hit this path:
			//   1. school-B admin login (step 4 of Run)
			//   2. link-email login during post-link verification (step 7)
			// We route by request body so each can be failed independently.
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if opts.verifyLoginStatus != 0 && body["email"] == "demo1@mail.de" {
				w.WriteHeader(opts.verifyLoginStatus)
				_, _ = fmt.Fprint(w, `{"error":"login failed"}`)
				return
			}
			// LoginTenant in phoenixapi expects access_token at the top
			// level (not nested under "data"), matching the real
			// /auth/login response shape.
			_ = json.NewEncoder(w).Encode(map[string]string{
				"access_token":  "tenant-token",
				"refresh_token": "refresh",
			})

		case "/auth/account/tenants":
			if opts.verifyTenantsStatus != 0 {
				w.WriteHeader(opts.verifyTenantsStatus)
				_, _ = fmt.Fprint(w, `{"error":"server error"}`)
				return
			}
			// Default: both tenants present (the post-link healthy state).
			// Tests that want to simulate a missing mapping pass a
			// `verifyTenantSlugs` override.
			slugs := []string{"demo-school", "second-school"}
			if opts.verifyTenantSlugs != nil {
				slugs = *opts.verifyTenantSlugs
			}
			data := make([]map[string]any, 0, len(slugs))
			for _, slug := range slugs {
				data = append(data, map[string]any{"slug": slug})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": data})

		case "/operator/schools":
			if r.Method == "GET" {
				slug := opts.schoolListSlug
				if slug == "" {
					slug = "second-school"
				}
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": []map[string]any{
						{"id": 99, "slug": slug},
					},
				})
				return
			}
			if opts.createSchoolStatus != 0 {
				w.WriteHeader(opts.createSchoolStatus)
				_, _ = fmt.Fprint(w, `{"error":"already exists"}`)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"id": 42, "slug": "second-school"},
			})

		case "/operator/schools/42/invite-admin", "/operator/schools/99/invite-admin":
			if opts.inviteStatus != 0 {
				w.WriteHeader(opts.inviteStatus)
				_, _ = fmt.Fprint(w, `{"error":"already invited"}`)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"token": "second-invite-token"},
			})

		case "/auth/invitations/second-invite-token/accept":
			if opts.acceptStatus != 0 {
				w.WriteHeader(opts.acceptStatus)
				_, _ = fmt.Fprint(w, `{"error":"already accepted"}`)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"account_id": 7},
			})

		case "/auth/roles":
			if opts.rolesStatus != 0 {
				w.WriteHeader(opts.rolesStatus)
				_, _ = fmt.Fprint(w, `{"error":"server error"}`)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": 1, "name": "admin"},
					{"id": 2, "name": "user"},
				},
			})

		case "/auth/link-to-tenant":
			if opts.linkStatus != 0 {
				w.WriteHeader(opts.linkStatus)
				_, _ = fmt.Fprint(w, `{"error":"linking failed"}`)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"account_id": 7},
			})

		default:
			t.Logf("unmocked path: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// secondTenantTestRuntime returns a Runtime ready to feed into
// secondTenantStep.Run. The runtime has an operator-authenticated client
// pointing at `srv` and a populated bootstrap state so the step's
// preconditions are satisfied.
func secondTenantTestRuntime(t *testing.T, srv *httptest.Server, opts *SecondTenantOptions) (*Runtime, *Seeder) {
	t.Helper()
	// StaffPassword is required by the post-link verification step to
	// log in as opts.LinkEmail. Real callers always set it via
	// scripts/e2e.sh; tests must mirror that.
	seeder := NewSeeder(srv.URL, false, SeedOptions{
		SecondTenant:  opts,
		StaffPassword: "Test1234%",
	})
	rt := newRuntime(seeder, "operator@e2e.local", "Op1234%", "1234")
	rt.OperatorAuth = phoenixapi.AuthRef{Kind: phoenixapi.AuthBearer, Token: "op-token"}
	rt.TenantAuth = phoenixapi.AuthRef{Kind: phoenixapi.AuthBearer, Token: "tenant-token"}
	rt.Bootstrap = &bootstrapSeedState{
		OrganizationID: 1,
		SchoolID:       2,
		SchoolSlug:     "demo-school",
		TenantSlug:     "demo-school",
	}
	return rt, seeder
}

func standardSecondTenantOpts() *SecondTenantOptions {
	return &SecondTenantOptions{
		Slug:          "second-school",
		Name:          "Demo School 2",
		AdminEmail:    "admin-b@e2e.local",
		AdminPassword: "Test1234%",
		LinkEmail:     "demo1@mail.de",
	}
}

func TestSecondTenantStep_HappyPath(t *testing.T) {
	srv := secondTenantMock(t, secondTenantMockOpts{})
	defer srv.Close()

	opts := standardSecondTenantOpts()
	rt, seeder := secondTenantTestRuntime(t, srv, opts)

	step := secondTenantStep{seeder: seeder}
	require.NoError(t, step.Run(context.Background(), rt))

	require.NotNil(t, rt.SecondTenant)
	assert.Equal(t, int64(42), rt.SecondTenant.SchoolID)
	assert.Equal(t, "second-school", rt.SecondTenant.Slug)
	assert.Equal(t, "demo1@mail.de", rt.SecondTenant.LinkEmail)
}

func TestSecondTenantStep_SchoolAlreadyExists_FallsThroughToLookup(t *testing.T) {
	srv := secondTenantMock(t, secondTenantMockOpts{
		createSchoolStatus: http.StatusConflict,
		schoolListSlug:     "second-school",
	})
	defer srv.Close()

	rt, seeder := secondTenantTestRuntime(t, srv, standardSecondTenantOpts())
	require.NoError(t, secondTenantStep{seeder: seeder}.Run(context.Background(), rt))

	// School ID comes from the listing endpoint (99), not the create one (42).
	require.NotNil(t, rt.SecondTenant)
	assert.Equal(t, int64(99), rt.SecondTenant.SchoolID)
}

func TestSecondTenantStep_SchoolCreate500_HardFails(t *testing.T) {
	// This is the whole point of moving the logic out of bash: a 500 here
	// MUST NOT be silently treated as "already exists, continuing". It
	// must surface as a real error so the developer sees the contract or
	// environment is broken.
	srv := secondTenantMock(t, secondTenantMockOpts{
		createSchoolStatus: http.StatusInternalServerError,
	})
	defer srv.Close()

	rt, seeder := secondTenantTestRuntime(t, srv, standardSecondTenantOpts())
	err := secondTenantStep{seeder: seeder}.Run(context.Background(), rt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create second school")
}

func TestSecondTenantStep_InviteAlreadyAccepted_SkipsAccept(t *testing.T) {
	srv := secondTenantMock(t, secondTenantMockOpts{
		inviteStatus: http.StatusConflict,
	})
	defer srv.Close()

	rt, seeder := secondTenantTestRuntime(t, srv, standardSecondTenantOpts())
	require.NoError(t, secondTenantStep{seeder: seeder}.Run(context.Background(), rt))
	require.NotNil(t, rt.SecondTenant)
}

func TestSecondTenantStep_LinkConflict_TreatedIdempotent(t *testing.T) {
	srv := secondTenantMock(t, secondTenantMockOpts{
		linkStatus: http.StatusConflict,
	})
	defer srv.Close()

	rt, seeder := secondTenantTestRuntime(t, srv, standardSecondTenantOpts())
	// 409 on link is the canonical "demo1 is already mapped" state from
	// any re-run after the first; the step must accept it silently.
	require.NoError(t, secondTenantStep{seeder: seeder}.Run(context.Background(), rt))
}

func TestSecondTenantStep_Link500_HardFails(t *testing.T) {
	// The bash script previously swallowed *every* link-step error as
	// "already linked (server response: ...)". This test pins down the
	// new contract: only 409 is silent; everything else is loud.
	srv := secondTenantMock(t, secondTenantMockOpts{
		linkStatus: http.StatusInternalServerError,
	})
	defer srv.Close()

	rt, seeder := secondTenantTestRuntime(t, srv, standardSecondTenantOpts())
	err := secondTenantStep{seeder: seeder}.Run(context.Background(), rt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "link demo1@mail.de into second-school")
}

func TestSecondTenantStep_Link400_HardFails(t *testing.T) {
	// 400 is what would surface for a malformed payload or a contract
	// drift on the backend side; just as bad to swallow as a 500.
	srv := secondTenantMock(t, secondTenantMockOpts{
		linkStatus: http.StatusBadRequest,
	})
	defer srv.Close()

	rt, seeder := secondTenantTestRuntime(t, srv, standardSecondTenantOpts())
	err := secondTenantStep{seeder: seeder}.Run(context.Background(), rt)
	require.Error(t, err)
}

func TestSecondTenantStep_Roles500_HardFails(t *testing.T) {
	srv := secondTenantMock(t, secondTenantMockOpts{
		rolesStatus: http.StatusInternalServerError,
	})
	defer srv.Close()

	rt, seeder := secondTenantTestRuntime(t, srv, standardSecondTenantOpts())
	err := secondTenantStep{seeder: seeder}.Run(context.Background(), rt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetch roles")
}

func TestSecondTenantStep_MissingBootstrap(t *testing.T) {
	srv := secondTenantMock(t, secondTenantMockOpts{})
	defer srv.Close()

	rt, seeder := secondTenantTestRuntime(t, srv, standardSecondTenantOpts())
	rt.Bootstrap = nil
	err := secondTenantStep{seeder: seeder}.Run(context.Background(), rt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bootstrap state not available")
}

func TestSecondTenantStep_InvalidOptions(t *testing.T) {
	srv := secondTenantMock(t, secondTenantMockOpts{})
	defer srv.Close()

	rt, seeder := secondTenantTestRuntime(t, srv, &SecondTenantOptions{
		// missing required fields
		Slug: "second-school",
	})
	err := secondTenantStep{seeder: seeder}.Run(context.Background(), rt)
	require.Error(t, err)
}

// =============================================================================
// Post-link verification step (verifyLinkEmailHasBothTenants)
// =============================================================================

func TestSecondTenantStep_LinkVerification_BothTenantsPresent_OK(t *testing.T) {
	// Sanity: the happy-path mock default returns both slugs, so
	// the verification step must succeed without complaint.
	srv := secondTenantMock(t, secondTenantMockOpts{})
	defer srv.Close()

	rt, seeder := secondTenantTestRuntime(t, srv, standardSecondTenantOpts())
	require.NoError(t, secondTenantStep{seeder: seeder}.Run(context.Background(), rt))
}

func TestSecondTenantStep_LinkVerification_MissingSecondSlug_HardFails(t *testing.T) {
	// Simulates the contract-drift case the reviewer was worried about:
	// the link API silently succeeds (200) but the mapping never
	// actually persists. Without a positive read-back this would slip
	// through and surface later as a flaky tenant-switch failure.
	onlyBootstrap := []string{"demo-school"}
	srv := secondTenantMock(t, secondTenantMockOpts{
		verifyTenantSlugs: &onlyBootstrap,
	})
	defer srv.Close()

	rt, seeder := secondTenantTestRuntime(t, srv, standardSecondTenantOpts())
	err := secondTenantStep{seeder: seeder}.Run(context.Background(), rt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing mapping to second tenant")
	assert.Contains(t, err.Error(), "second-school")
}

func TestSecondTenantStep_LinkVerification_MissingBootstrapSlug_HardFails(t *testing.T) {
	// Pathological state: link account somehow lost its bootstrap-tenant
	// mapping. Should never happen in normal operation, but the
	// verification step asserts BOTH slugs so a regression here is loud.
	onlySecond := []string{"second-school"}
	srv := secondTenantMock(t, secondTenantMockOpts{
		verifyTenantSlugs: &onlySecond,
	})
	defer srv.Close()

	rt, seeder := secondTenantTestRuntime(t, srv, standardSecondTenantOpts())
	err := secondTenantStep{seeder: seeder}.Run(context.Background(), rt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing mapping to bootstrap tenant")
	assert.Contains(t, err.Error(), "demo-school")
}

func TestSecondTenantStep_LinkVerification_NoTenantsReturned_HardFails(t *testing.T) {
	// Worst-case: the read-back returns an empty list. Both asserts
	// must fail; we surface the bootstrap-mapping miss first because
	// the workflow checks them in order, but either error is acceptable
	// proof that the verification refused to pass through.
	none := []string{}
	srv := secondTenantMock(t, secondTenantMockOpts{
		verifyTenantSlugs: &none,
	})
	defer srv.Close()

	rt, seeder := secondTenantTestRuntime(t, srv, standardSecondTenantOpts())
	err := secondTenantStep{seeder: seeder}.Run(context.Background(), rt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing mapping")
}

func TestSecondTenantStep_LinkVerification_LoginFails_HardFails(t *testing.T) {
	// If demo1 cannot log in at all after the link step, the link must
	// have broken the account in some way (or the password contract
	// drifted). Either way, fail loudly rather than write a seed-state
	// that the tenant-switch spec will then mysteriously trip over.
	srv := secondTenantMock(t, secondTenantMockOpts{
		verifyLoginStatus: http.StatusUnauthorized,
	})
	defer srv.Close()

	rt, seeder := secondTenantTestRuntime(t, srv, standardSecondTenantOpts())
	err := secondTenantStep{seeder: seeder}.Run(context.Background(), rt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "verification: login as demo1@mail.de")
}

func TestSecondTenantStep_LinkVerification_TenantsEndpoint500_HardFails(t *testing.T) {
	srv := secondTenantMock(t, secondTenantMockOpts{
		verifyTenantsStatus: http.StatusInternalServerError,
	})
	defer srv.Close()

	rt, seeder := secondTenantTestRuntime(t, srv, standardSecondTenantOpts())
	err := secondTenantStep{seeder: seeder}.Run(context.Background(), rt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GET /auth/account/tenants")
}

func TestSecondTenantStep_LinkVerification_MissingStaffPassword_HardFails(t *testing.T) {
	// If StaffPassword isn't set we cannot read back as the link
	// account. Rather than silently skipping the verification (which
	// would defeat the whole point of having it), we fail loudly so
	// the upstream caller sees the contract drift.
	srv := secondTenantMock(t, secondTenantMockOpts{})
	defer srv.Close()

	// Bypass the helper's StaffPassword default to reproduce the empty case.
	seeder := NewSeeder(srv.URL, false, SeedOptions{SecondTenant: standardSecondTenantOpts()})
	rt := newRuntime(seeder, "operator@e2e.local", "Op1234%", "1234")
	rt.OperatorAuth = phoenixapi.AuthRef{Kind: phoenixapi.AuthBearer, Token: "op-token"}
	rt.TenantAuth = phoenixapi.AuthRef{Kind: phoenixapi.AuthBearer, Token: "tenant-token"}
	rt.Bootstrap = &bootstrapSeedState{
		OrganizationID: 1,
		SchoolID:       2,
		SchoolSlug:     "demo-school",
		TenantSlug:     "demo-school",
	}

	err := secondTenantStep{seeder: seeder}.Run(context.Background(), rt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SeedOptions.StaffPassword is empty")
}

func TestFullDemoWorkflow_SchedulesSecondTenantStepWhenOptSet(t *testing.T) {
	withOpts := NewSeeder("http://localhost:8080", false, SeedOptions{
		SecondTenant: standardSecondTenantOpts(),
	})
	withWorkflow := fullDemoWorkflow(withOpts)
	assert.True(t, workflowHasStep(withWorkflow, "Second tenant provisioning"),
		"workflow with SecondTenant opts must include the provisioning step")

	withoutOpts := NewSeeder("http://localhost:8080", false, SeedOptions{})
	withoutWorkflow := fullDemoWorkflow(withoutOpts)
	assert.False(t, workflowHasStep(withoutWorkflow, "Second tenant provisioning"),
		"default workflow must not include the second-tenant step")
}

func TestFullDemoWorkflow_E2EScenarioAppliesSecondTenantDefaults(t *testing.T) {
	seeder := NewSeeder("http://localhost:8080", false, SeedOptions{Scenario: SeedScenarioE2E})

	require.NotNil(t, seeder.options.SecondTenant)
	withWorkflow := fullDemoWorkflow(seeder)
	assert.True(t, workflowHasStep(withWorkflow, "Second tenant provisioning"),
		"e2e scenario must schedule the provisioning step via seedapi defaults, not only via the CLI")
}

func workflowHasStep(w Workflow, name string) bool {
	for _, s := range w.Steps {
		if s.Name() == name {
			return true
		}
	}
	return false
}
