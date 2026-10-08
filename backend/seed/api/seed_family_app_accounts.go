package api

import (
	"context"
	"fmt"
	"slices"
)

// familiesWithoutApp are the demo children whose families stay without the
// parents app: the re-enrollment overview (#3379) shows a family that can
// only be reached by phone, and "Neue Nachricht" shows the hint that a child
// has no parent access. Both need a real example, not a whole school of them.
var familiesWithoutApp = map[int]bool{24: true, 47: true}

// openInvitationsKept is how many parents of approved online enrollments keep
// their invitation open: the school still sees one waiting for an answer.
const openInvitationsKept = 2

// seedFamilyAppAccountsStep gives one parent of every other demo child an
// account in the parents app, and lets the parents of approved online
// enrollments accept the invitation their approval sent. Before, only the six
// scripted parents had one, so "Neue Nachricht" offered no recipient for
// almost every child (#3894). The accounts only exist; the scripted parents
// stay the ones who act.
type seedFamilyAppAccountsStep struct {
	seeder *Seeder
}

func (seedFamilyAppAccountsStep) Name() string { return "Seeding parent app accounts" }

func (s seedFamilyAppAccountsStep) Run(ctx context.Context, rt *Runtime) error {
	if rt.FixedSeeder == nil {
		return fmt.Errorf("demo guardians not available")
	}
	adminAuth, err := rt.Adapter.LoginTenant(ctx, rt.Bootstrap.AdminEmail, rt.Bootstrap.AdminPassword, rt.Bootstrap.TenantSlug)
	if err != nil {
		return fmt.Errorf("login seed school admin: %w", err)
	}
	rt.SetTenantAuth(adminAuth)
	step := parentEnrollmentSeedStep(s)
	password, err := step.parentPassword()
	if err != nil {
		return err
	}
	guardians, err := familyAppGuardians(rt.FixedSeeder.guardianIDs, rt.Parents)
	if err != nil {
		return err
	}
	invited, err := enrollmentInvitationGuardians(rt, adminAuth)
	if err != nil {
		return err
	}
	guardians = append(guardians, invited...)
	for _, guardianID := range guardians {
		// For an open invitation the invite returns that one with its token.
		token, err := step.inviteGuardian(rt, adminAuth, guardianID)
		if err != nil {
			return fmt.Errorf("invite guardian %d: %w", guardianID, err)
		}
		if _, err := step.acceptGuardianInvitation(rt, token, password); err != nil {
			return fmt.Errorf("accept guardian invitation %d: %w", guardianID, err)
		}
	}
	fmt.Printf("  %d parent app accounts created\n", len(guardians))
	return nil
}

// familyAppGuardians returns the primary guardian of each demo child whose
// family has no app account yet, in DemoStudents order, leaving out
// familiesWithoutApp.
func familyAppGuardians(guardianIDs map[string]int64, parents []ParentCredentials) ([]int64, error) {
	withAccount := make(map[int64]bool, len(parents))
	for _, parent := range parents {
		withAccount[parent.GuardianID] = true
	}
	covered := map[int]bool{}
	for _, guardian := range DemoGuardians {
		if withAccount[guardianIDs[guardian.FirstName+" "+guardian.LastName]] {
			covered[guardian.StudentIndex] = true
		}
	}
	result := make([]int64, 0, len(DemoStudents))
	for _, guardian := range DemoGuardians {
		if !guardian.IsPrimary || covered[guardian.StudentIndex] || familiesWithoutApp[guardian.StudentIndex] {
			continue
		}
		key := guardian.FirstName + " " + guardian.LastName
		guardianID, ok := guardianIDs[key]
		if !ok || guardianID == 0 {
			return nil, fmt.Errorf("guardian %s was not created", key)
		}
		result = append(result, guardianID)
		covered[guardian.StudentIndex] = true
	}
	return result, nil
}

// enrollmentInvitationGuardians returns the guardians whose invitation is
// still open, except the openInvitationsKept lowest ids.
func enrollmentInvitationGuardians(rt *Runtime, auth AuthRef) ([]int64, error) {
	raw, err := rt.Client.GetWithAuth(auth, "/api/guardians/invitations/pending")
	if err != nil {
		return nil, fmt.Errorf("list open guardian invitations: %w", err)
	}
	var resp struct {
		Data []struct {
			GuardianProfileID int64 `json:"guardian_profile_id"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &resp); err != nil {
		return nil, fmt.Errorf("parse open guardian invitations: %w", err)
	}
	ids := make([]int64, 0, len(resp.Data))
	for _, invitation := range resp.Data {
		ids = append(ids, invitation.GuardianProfileID)
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	if len(ids) <= openInvitationsKept {
		return nil, nil
	}
	return ids[openInvitationsKept:], nil
}
