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
var familiesWithoutApp = map[int]bool{24: true, 48: true}

// openInvitationsKept is how many parents of approved online enrollments keep
// their invitation open: the school still sees one waiting for an answer.
const openInvitationsKept = 2

// seedFamilyAppAccountsStep gives one parent of every other demo child an
// account in the parents app, and lets the parents of approved online
// enrollments accept an invitation. Before, only the six
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
	guardians, err := familyAppGuardianIndexes(rt.FixedSeeder.guardianIDs, rt.Parents)
	if err != nil {
		return err
	}
	invitations, err := enrollmentInvitationsToAccept(rt, adminAuth)
	if err != nil {
		return err
	}
	if err := createFamilyAccounts(rt, step, adminAuth, guardians, password); err != nil {
		return err
	}
	// The approval's own link is stored as hash only (#4902) and reaches
	// nobody but the parent's mailbox: the seeder accepts a fresh invitation
	// of the same guardian, issued through the seed-only token exposure.
	for _, invitation := range invitations {
		token, err := step.inviteGuardian(rt, adminAuth, invitation.GuardianProfileID)
		if err != nil {
			return fmt.Errorf("invite guardian %d: %w", invitation.GuardianProfileID, err)
		}
		if _, err := step.acceptGuardianInvitation(rt, token, password); err != nil {
			return fmt.Errorf("accept guardian invitation %d: %w", invitation.GuardianProfileID, err)
		}
	}
	fmt.Printf("  %d parent app accounts created\n", len(guardians)+len(invitations))
	return nil
}

// createFamilyAccounts invites and accepts an account for each guardian and
// keeps the credentials of those with full parent rights in the runtime.
func createFamilyAccounts(rt *Runtime, step parentEnrollmentSeedStep, adminAuth AuthRef, guardians []int, password string) error {
	rt.FamilyAccounts = rt.FamilyAccounts[:0]
	for _, index := range guardians {
		guardian := DemoGuardians[index]
		guardianID := rt.FixedSeeder.guardianIDs[guardian.FirstName+" "+guardian.LastName]
		token, err := step.inviteGuardian(rt, adminAuth, guardianID)
		if err != nil {
			return fmt.Errorf("invite guardian %d: %w", guardianID, err)
		}
		accountID, err := step.acceptGuardianInvitation(rt, token, password)
		if err != nil {
			return fmt.Errorf("accept guardian invitation %d: %w", guardianID, err)
		}
		if account, ok := familyAccount(rt, index, guardianID, accountID, password); ok {
			rt.FamilyAccounts = append(rt.FamilyAccounts, account)
		}
	}
	return nil
}

// familyAccount returns the credentials of a family app account, so later
// steps can act as the family. Guardians with a limited role (pickup only,
// custom) have an account but no full parent rights; they stay out.
func familyAccount(rt *Runtime, guardianIndex int, guardianID, accountID int64, password string) (ParentCredentials, bool) {
	guardian := DemoGuardians[guardianIndex]
	studentID := rt.FixedSeeder.studentIDByIndex[guardian.StudentIndex]
	if demoGuardianHasLimitedRole(guardianIndex) || studentID == 0 {
		return ParentCredentials{}, false
	}
	return ParentCredentials{
		Email:      scopedEmail(guardian.Email, rt.FixedSeeder.accountScope),
		Password:   password,
		Name:       guardian.FirstName + " " + guardian.LastName,
		AccountID:  accountID,
		GuardianID: guardianID,
		StudentIDs: []int64{studentID},
	}, true
}

// familyAppGuardians returns the primary guardian of each demo child whose
// family has no app account yet, in DemoStudents order, leaving out
// familiesWithoutApp.
func familyAppGuardians(guardianIDs map[string]int64, parents []ParentCredentials) ([]int64, error) {
	indexes, err := familyAppGuardianIndexes(guardianIDs, parents)
	if err != nil {
		return nil, err
	}
	result := make([]int64, 0, len(indexes))
	for _, index := range indexes {
		guardian := DemoGuardians[index]
		result = append(result, guardianIDs[guardian.FirstName+" "+guardian.LastName])
	}
	return result, nil
}

// familyAppGuardianIndexes is familyAppGuardians as DemoGuardians indexes.
func familyAppGuardianIndexes(guardianIDs map[string]int64, parents []ParentCredentials) ([]int, error) {
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
	result := make([]int, 0, len(DemoStudents))
	for index, guardian := range DemoGuardians {
		if !guardian.IsPrimary || covered[guardian.StudentIndex] || familiesWithoutApp[guardian.StudentIndex] {
			continue
		}
		key := guardian.FirstName + " " + guardian.LastName
		if guardianIDs[key] == 0 {
			return nil, fmt.Errorf("guardian %s was not created", key)
		}
		result = append(result, index)
		covered[guardian.StudentIndex] = true
	}
	return result, nil
}

type enrollmentInvitation struct {
	ID                int64 `json:"id"`
	GuardianProfileID int64 `json:"guardian_profile_id"`
}

// enrollmentInvitationsToAccept returns the redeemable invitations whose
// guardians get an account, keeping two guardians' invitations open for the
// demo overview.
func enrollmentInvitationsToAccept(rt *Runtime, auth AuthRef) ([]enrollmentInvitation, error) {
	raw, err := rt.Client.GetWithAuth(auth, "/api/guardians/invitations/pending")
	if err != nil {
		return nil, fmt.Errorf("list open guardian invitations: %w", err)
	}
	var resp struct {
		Data []enrollmentInvitation `json:"data"`
	}
	if err := parseJSON(raw, &resp); err != nil {
		return nil, fmt.Errorf("parse open guardian invitations: %w", err)
	}
	slices.SortFunc(resp.Data, func(left, right enrollmentInvitation) int {
		switch {
		case left.GuardianProfileID < right.GuardianProfileID:
			return -1
		case left.GuardianProfileID > right.GuardianProfileID:
			return 1
		case left.ID < right.ID:
			return -1
		case left.ID > right.ID:
			return 1
		default:
			return 0
		}
	})
	keptGuardians := make(map[int64]bool, openInvitationsKept)
	result := make([]enrollmentInvitation, 0, len(resp.Data))
	for _, invitation := range resp.Data {
		if len(keptGuardians) < openInvitationsKept {
			keptGuardians[invitation.GuardianProfileID] = true
		}
		if keptGuardians[invitation.GuardianProfileID] {
			continue
		}
		result = append(result, invitation)
	}
	return result, nil
}
