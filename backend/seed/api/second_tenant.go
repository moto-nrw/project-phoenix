package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/moto-nrw/project-phoenix/integration/phoenixapi"
)

// SecondTenantOptions configures the second-tenant provisioning step. The
// step runs after the main demo seed completes, under the same operator
// authentication, and creates a sibling school under the existing
// organization plus an account-tenant link so the named tenant-switch
// account ends up with active mappings to both schools.
//
// Idempotency: the step uses the existing API and tolerates 409 Conflict
// at school-create, invitation-accept, and link-to-tenant. Any other
// non-2xx response (400, 401, 403, 5xx) aborts the run with a clear error.
// This is the whole point of moving the logic out of bash: bash's
// "any error means try the fallback" pattern silently swallowed real
// failures (validation, contract drift, 500s) and only surfaced them
// later as confusing tenant-switch test failures.
type SecondTenantOptions struct {
	// Slug for the second school (e.g. "second-school"). Must be a valid
	// DNS label. The school is created under the same organization the
	// main demo seed bootstrapped.
	Slug string
	// Display name (e.g. "Demo School 2").
	Name string
	// Email for the second-school admin (used to log in and call
	// /auth/link-to-tenant on behalf of school B).
	AdminEmail string
	// Password for the second-school admin. Must satisfy
	// ValidatePasswordStrength.
	AdminPassword string
	// LinkEmail is the existing account that gets a mapping into the
	// second school (typically demo1@mail.de, the OGS-Büro admin from
	// the main seed). After the link, this account has access to BOTH
	// tenants — the precondition for the TenantSwitcher dropdown to
	// render in the frontend.
	LinkEmail string
}

// SeedStateSecondTenant is the internal handoff object the provisioning
// step passes to the E2E manifest builder. It is NOT serialized directly;
// buildE2EManifest folds it into the dedicated Playwright contract so the
// harness reads a single machine-readable artifact.
type SeedStateSecondTenant struct {
	OrganizationID int64  `json:"organization_id"`
	SchoolID       int64  `json:"school_id"`
	Slug           string `json:"slug"`
	Name           string `json:"name"`
	AdminEmail     string `json:"admin_email"`
	AdminPassword  string `json:"admin_password"`
	// LinkEmail is the account that now has access to BOTH tenants.
	LinkEmail string `json:"link_email"`
}

// secondTenantStep runs after the main seed and provisions the second
// school + admin + account link. It is wired into the workflow only when
// SeedOptions.SecondTenant is non-nil so the default `seed` invocation
// remains a single-tenant bootstrap.
type secondTenantStep struct {
	seeder *Seeder
}

func (secondTenantStep) Name() string { return "Second tenant provisioning" }

func (s secondTenantStep) Run(ctx context.Context, rt *Runtime) error {
	if rt.Bootstrap == nil {
		return fmt.Errorf("bootstrap state not available — main seed must run first")
	}
	if s.seeder.options.SecondTenant == nil {
		return fmt.Errorf("SecondTenant options not provided — step should not have been scheduled")
	}
	opts := s.seeder.options.SecondTenant

	if err := opts.validate(); err != nil {
		return err
	}

	// All eight steps below run under the operator auth from the bootstrap
	// step (we restore tenant auth in defer so subsequent workflow steps
	// like simulator-config still see the school admin context).
	previousAuth := rt.TenantAuth
	rt.Client.BindAuth(rt.OperatorAuth)
	defer rt.Client.BindAuth(previousAuth)

	// 1. Create the second school under the existing organization.
	schoolID, err := s.findOrCreateSchool(rt, opts)
	if err != nil {
		return err
	}

	// 2. Invite an admin for the second school. 409 = invite already
	// consumed by a previous run; we'll just log in as the admin below.
	inviteToken, err := s.inviteSecondAdmin(rt, schoolID, opts)
	if err != nil {
		return err
	}

	// 3. Accept the invitation. Skipped when invite-step indicated the
	// admin already exists.
	if inviteToken != "" {
		if err := s.acceptInvitation(rt, inviteToken, opts); err != nil {
			return err
		}
	}

	// 4. Log in as school-B admin so we can call /auth/link-to-tenant
	// against school B's tenant context.
	//
	// This login also serves as positive verification of the previous
	// invite + accept steps: if either had silently failed (e.g., the
	// 409-tolerance branch swallowed a real error a future refactor
	// introduces), the account would not exist and this login would
	// surface the failure with a clear "login as second-school admin
	// (admin-b@e2e.local): ..." message before any link-step runs.
	bAuth, err := rt.Adapter.LoginTenant(ctx, opts.AdminEmail, opts.AdminPassword, opts.Slug)
	if err != nil {
		return fmt.Errorf("login as second-school admin (%s): %w", opts.AdminEmail, err)
	}
	rt.Client.BindAuth(bAuth)

	// 5. Resolve the admin role id within school B (role ids are scoped
	// to the tenant, so we can't reuse the operator's idea of them).
	adminRoleID, err := s.resolveAdminRoleID(rt)
	if err != nil {
		return err
	}

	// 6. Link the named account into school B. 409 = already linked
	// (re-run after a fresh seed); anything else hard-errors.
	if err := s.linkAccountToSecondTenant(rt, opts, adminRoleID); err != nil {
		return err
	}

	// 7. Belt-and-suspenders verification: log in as the link-email
	// account and read back its tenant mappings. Asserts BOTH the
	// bootstrap school slug and the second school slug are present.
	//
	// The reviewer asked for this explicitly: "alternativ sollte danach
	// explizit geprüft werden, dass demo1@mail.de wirklich Zugriff auf
	// beide Tenants hat". The HTTP-409-only check on the link step
	// already differentiates real failures from idempotent re-runs, but
	// the backend's LinkAccountToTenant is itself idempotent (returns
	// 200 even when already-linked) — so the API status alone is not
	// proof that a mapping exists. Only a positive read-back is.
	if err := s.verifyLinkEmailHasBothTenants(ctx, rt, opts); err != nil {
		return err
	}

	// 8. Stash everything an E2E spec might need for the tenant-switch
	// flow on the runtime so the E2E manifest writer can serialise it. The link
	// email is the canonical "this account now has 2+ tenants" signal.
	rt.SecondTenant = &SeedStateSecondTenant{
		OrganizationID: rt.Bootstrap.OrganizationID,
		SchoolID:       schoolID,
		Slug:           opts.Slug,
		Name:           opts.Name,
		AdminEmail:     opts.AdminEmail,
		AdminPassword:  opts.AdminPassword,
		LinkEmail:      opts.LinkEmail,
	}

	fmt.Printf("Second tenant provisioned: %s (school_id=%d), %s now has access to both\n",
		opts.Slug, schoolID, opts.LinkEmail)
	return nil
}

func (o *SecondTenantOptions) validate() error {
	if o.Slug == "" {
		return fmt.Errorf("SecondTenant.Slug is required")
	}
	if o.Name == "" {
		return fmt.Errorf("SecondTenant.Name is required")
	}
	if o.AdminEmail == "" {
		return fmt.Errorf("SecondTenant.AdminEmail is required")
	}
	if o.AdminPassword == "" {
		return fmt.Errorf("SecondTenant.AdminPassword is required")
	}
	if o.LinkEmail == "" {
		return fmt.Errorf("SecondTenant.LinkEmail is required")
	}
	return nil
}

func (s secondTenantStep) findOrCreateSchool(rt *Runtime, opts *SecondTenantOptions) (int64, error) {
	resp, err := rt.Client.Post("/operator/schools", map[string]any{
		"organization_id": rt.Bootstrap.OrganizationID,
		"name":            opts.Name,
		"slug":            opts.Slug,
		"subdomain":       opts.Slug,
	})
	if err == nil {
		var payload struct {
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		if perr := parseJSON(resp, &payload); perr != nil {
			return 0, fmt.Errorf("parse second-school response: %w", perr)
		}
		if payload.Data.ID == 0 {
			return 0, fmt.Errorf("second-school create returned id=0 (response: %s)", string(resp))
		}
		return payload.Data.ID, nil
	}

	// Only 409 Conflict is allowed to fall through to the lookup path.
	// 400/401/403/5xx mean the contract or environment is broken; treat
	// them as hard errors with the upstream message intact.
	if !isConflict(err) {
		return 0, fmt.Errorf("create second school %q: %w", opts.Slug, err)
	}

	// Conflict — find the existing school by slug under our organization.
	listResp, lerr := rt.Client.Get(fmt.Sprintf("/operator/schools?organization_id=%d", rt.Bootstrap.OrganizationID))
	if lerr != nil {
		return 0, fmt.Errorf("list schools after 409: %w", lerr)
	}
	var listPayload struct {
		Data []struct {
			ID   int64  `json:"id"`
			Slug string `json:"slug"`
		} `json:"data"`
	}
	if perr := parseJSON(listResp, &listPayload); perr != nil {
		return 0, fmt.Errorf("parse schools list: %w", perr)
	}
	for _, school := range listPayload.Data {
		if school.Slug == opts.Slug {
			return school.ID, nil
		}
	}
	return 0, fmt.Errorf("second school %q not found in org %d after 409", opts.Slug, rt.Bootstrap.OrganizationID)
}

// inviteSecondAdmin returns the invitation token if a fresh invite was
// sent, or an empty string when the admin already exists (409 — re-run
// after a previous seed). All other errors abort.
func (s secondTenantStep) inviteSecondAdmin(rt *Runtime, schoolID int64, opts *SecondTenantOptions) (string, error) {
	resp, err := rt.Client.PostWithHeaders(
		fmt.Sprintf("/operator/schools/%d/invite-admin", schoolID),
		map[string]any{
			"email":      opts.AdminEmail,
			"first_name": "School",
			"last_name":  "BAdmin",
			"position":   "OGS-Büro",
		},
		map[string]string{seedTokenHeader: "true"},
	)
	if err == nil {
		var payload struct {
			Data struct {
				Token string `json:"token"`
			} `json:"data"`
		}
		if perr := parseJSON(resp, &payload); perr != nil {
			return "", fmt.Errorf("parse invite response: %w", perr)
		}
		if payload.Data.Token == "" {
			return "", fmt.Errorf("invite response missing token (X-Phoenix-Seed-Token header may have been ignored)")
		}
		return payload.Data.Token, nil
	}
	if isConflict(err) {
		// Admin invite already accepted on a previous run — login below
		// will pick up the existing account.
		return "", nil
	}
	return "", fmt.Errorf("invite second-school admin %s: %w", opts.AdminEmail, err)
}

func (s secondTenantStep) acceptInvitation(rt *Runtime, token string, opts *SecondTenantOptions) error {
	_, err := rt.Client.PostPublic(
		fmt.Sprintf("/auth/invitations/%s/accept", token),
		map[string]any{
			"password":         opts.AdminPassword,
			"confirm_password": opts.AdminPassword,
		},
	)
	if err == nil {
		return nil
	}
	// 409 here means the invitation was already accepted — usually a
	// re-run with the seed token still in flight. Anything else is real.
	if isConflict(err) {
		return nil
	}
	return fmt.Errorf("accept second-school invitation: %w", err)
}

func (s secondTenantStep) resolveAdminRoleID(rt *Runtime) (int64, error) {
	resp, err := rt.Client.Get("/auth/roles?name=admin")
	if err != nil {
		return 0, fmt.Errorf("fetch roles in school B: %w", err)
	}
	var payload struct {
		Data []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if perr := parseJSON(resp, &payload); perr != nil {
		return 0, fmt.Errorf("parse roles response: %w", perr)
	}
	for _, role := range payload.Data {
		if role.Name == "admin" {
			return role.ID, nil
		}
	}
	if len(payload.Data) > 0 {
		return payload.Data[0].ID, nil
	}
	return 0, fmt.Errorf("no admin role found in school B (response: %s)", string(resp))
}

func (s secondTenantStep) linkAccountToSecondTenant(rt *Runtime, opts *SecondTenantOptions, adminRoleID int64) error {
	_, err := rt.Client.Post("/auth/link-to-tenant", map[string]any{
		"email":   opts.LinkEmail,
		"role_id": adminRoleID,
	})
	if err == nil {
		return nil
	}
	if isConflict(err) {
		// Already mapped — every re-run after the first lands here.
		return nil
	}
	return fmt.Errorf("link %s into %s: %w", opts.LinkEmail, opts.Slug, err)
}

// verifyLinkEmailHasBothTenants logs in as opts.LinkEmail and asserts
// that GET /auth/account/tenants returns BOTH the bootstrap school slug
// and the second school slug. This is the positive read-back the
// reviewer requested: it catches the failure mode where the link API
// returns success but no mapping actually persists (e.g., a future
// repository change that double-deletes inactive mappings, or a
// silent transaction rollback). Pure HTTP-status checks cannot detect
// that — only a cross-tenant read can.
func (s secondTenantStep) verifyLinkEmailHasBothTenants(ctx context.Context, rt *Runtime, opts *SecondTenantOptions) error {
	staffPassword := s.seeder.options.StaffPassword
	if staffPassword == "" {
		// Without a known password we can't read back as the link account.
		// Fail loudly rather than silently skip — every E2E run sets
		// StaffPassword via the canonical Go-owned E2E scenario, so an empty value here
		// means an upstream contract has drifted.
		return fmt.Errorf("verification: cannot log in as %s — SeedOptions.StaffPassword is empty", opts.LinkEmail)
	}

	// No tenant_slug here: log in against the account's default mapping.
	// The /auth/account/tenants response then enumerates all active
	// mappings regardless of which one we authenticated against.
	linkAuth, err := rt.Adapter.LoginTenant(ctx, opts.LinkEmail, staffPassword, "")
	if err != nil {
		return fmt.Errorf("verification: login as %s after link: %w", opts.LinkEmail, err)
	}
	// Caller's deferred BindAuth(previousAuth) at the top of Run()
	// restores the prior auth when this step returns, so no explicit
	// restore is needed here.
	rt.Client.BindAuth(linkAuth)

	resp, err := rt.Client.Get("/auth/account/tenants")
	if err != nil {
		return fmt.Errorf("verification: GET /auth/account/tenants for %s: %w", opts.LinkEmail, err)
	}
	var payload struct {
		Data []struct {
			Slug string `json:"slug"`
		} `json:"data"`
	}
	if perr := parseJSON(resp, &payload); perr != nil {
		return fmt.Errorf("verification: parse tenants response: %w", perr)
	}

	seen := make(map[string]bool, len(payload.Data))
	slugs := make([]string, 0, len(payload.Data))
	for _, t := range payload.Data {
		seen[t.Slug] = true
		slugs = append(slugs, t.Slug)
	}
	if !seen[rt.Bootstrap.SchoolSlug] {
		return fmt.Errorf("verification: %s is missing mapping to bootstrap tenant %q after link (saw: %v)",
			opts.LinkEmail, rt.Bootstrap.SchoolSlug, slugs)
	}
	if !seen[opts.Slug] {
		return fmt.Errorf("verification: %s is missing mapping to second tenant %q after link (saw: %v)",
			opts.LinkEmail, opts.Slug, slugs)
	}
	return nil
}

// isConflict returns true iff err is a phoenixapi.APIError with status 409.
// Anything else (network error, 4xx other than 409, 5xx) returns false so
// the caller surfaces it as a real error.
func isConflict(err error) bool {
	var apiErr *phoenixapi.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusConflict
	}
	return false
}
