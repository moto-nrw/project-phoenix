package parentportal_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

// The pushes of a sent care contract and of a release for parents (#5941)
// over the real Betreuungsvertrag, Organisation & Tenancy, People Directory,
// Identity & Access and the parent audience projection, with the pushes
// recorded. The notices run as the Delivery worker runs them.

func traegerDocumentNotices(t *testing.T) (parentService.DocumentNotices, *recordedPushes, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	deps := documentInboxDependencies(t, db, &owners.CareContractTestMails{})
	people := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	organizations, err := owners.NewOrganizationTenancy(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	pushes := &recordedPushes{}
	deps.DocumentNotifier = pushes
	deps.StudentRepo = owners.NewParentStudentReads(people.Student)
	deps.ContractInboxNotices = owners.NewParentContractInboxNotices()
	deps.ReleaseNotices = owners.NewParentReleaseNotices()
	deps.ReleaseRecipients = owners.NewParentReleaseRecipients(organizations, owners.NewParentGroupAudience(testpkg.NewDatabase(t, db)))
	deps.Logger = slog.Default()
	return parentportalcompose.NewDocumentNotices(deps), pushes, db
}

// to keeps the pushes to one account: the shared school of the fixtures has
// the parents of other tests too.
func (r *recordedPushes) to(accountID int64) []parentService.BillingDocumentPush {
	var kept []parentService.BillingDocumentPush
	for _, push := range r.all() {
		if push.AccountID == accountID {
			kept = append(kept, push)
		}
	}
	return kept
}

func organizationOf(t *testing.T, db *bun.DB, schoolID int64) int64 {
	t.Helper()
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, schoolID).Scan(context.Background(), &organization))
	return organization
}

// TestContractNoticeNeedsTheRelationshipToTheContractsChild pins #5941 for
// contracts: the push goes to the account of the address the contract went
// to, about the contract's child only, while that child's relationship
// grants parent_portal.enrollments.view. A right for a sibling, another
// address, another Träger, a draft and a withdrawn, terminated or ended
// contract push nothing; the check runs again at every send.
func TestContractNoticeNeedsTheRelationshipToTheContractsChild(t *testing.T) {
	t.Parallel()
	notices, pushes, db := traegerDocumentNotices(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	sibling := testpkg.CreateTestStudent(t, db, "Mia", "Schneider", "1a")
	siblingLink := testpkg.CreateTestStudentGuardianLink(t, db, sibling.ID, chain.GuardianProfileID, "primary_guardian")
	organization := organizationOf(t, db, chain.TenantID)
	testpkg.OwnTraegerCareContracts(t, db, organization)
	felix := inboxContract(t, db, organization, chain.TenantID, chain.StudentID, "Felix", chain.Email)
	mia := inboxContract(t, db, organization, chain.TenantID, sibling.ID, "Mia", chain.Email)
	otherFamily := inboxContract(t, db, organization, chain.TenantID, chain.StudentID, "Tom", "andere.familie@example.org")
	draft := inboxContract(t, db, organization, chain.TenantID, chain.StudentID, "Lena", "")

	require.NoError(t, notices.NotifyContractDocument(ctx, organization, felix, chain.Email))
	require.Equal(t, []parentService.BillingDocumentPush{{
		Kind: parentService.DocumentPushContract, TenantID: chain.TenantID, AccountID: chain.AccountID,
		StudentIDs: []int64{chain.StudentID}, DocumentID: felix,
	}}, pushes.to(chain.AccountID), "the contract's child only, never its siblings")

	// The sibling's relationship keeps the portal but loses the inbox right;
	// Felix's right does not open Mia's contract.
	require.NoError(t, testpkg.SetTestStudentGuardianPermissions(ctx, db, siblingLink.ID, map[string]interface{}{"parent_portal.access": true}))
	require.NoError(t, notices.NotifyContractDocument(ctx, organization, mia, chain.Email))
	for name, notice := range map[string]struct {
		organization, contract int64
		email                  string
	}{
		"contract sent to another address": {organization, otherFamily, chain.Email},
		"draft":                            {organization, draft, chain.Email},
		"another Träger":                   {organization + 1_000_000, felix, chain.Email},
		"address without an account":       {organization, felix, "niemand@example.org"},
	} {
		require.NoError(t, notices.NotifyContractDocument(ctx, notice.organization, notice.contract, notice.email), name)
	}
	assert.Len(t, pushes.to(chain.AccountID), 1, "only the permitted child's contract pushed")

	for _, status := range []string{"widerrufen", "gekuendigt", "beendet"} {
		_, err := db.ExecContext(ctx, `UPDATE care_contract.contracts SET status = ?, concluded_at = COALESCE(concluded_at, NOW()),
			conclusion_method = COALESCE(conclusion_method, 'papier')
			WHERE organization_id = ? AND id = ?`, status, organization, felix)
		require.NoError(t, err)
		require.NoError(t, notices.NotifyContractDocument(ctx, organization, felix, chain.Email))
		assert.Len(t, pushes.to(chain.AccountID), 1, "a %s contract pushes nothing", status)
	}

	_, err := db.ExecContext(ctx, `UPDATE care_contract.contracts SET status = 'versendet', concluded_at = NULL, conclusion_method = NULL WHERE organization_id = ? AND id = ?`, organization, felix)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE auth.guardian_student_access SET permissions = '{"parent_portal.access": true}'::jsonb
		WHERE relationship_id IN (SELECT id FROM users.student_guardian_relationships WHERE guardian_profile_id = ?)`, chain.GuardianProfileID)
	require.NoError(t, err)
	require.NoError(t, notices.NotifyContractDocument(ctx, organization, felix, chain.Email))
	assert.Len(t, pushes.to(chain.AccountID), 1, "a right removed before the send stops the push")
}

// TestReleaseNoticeFollowsTheRelationshipAndTheActiveRelease pins #5941 for
// releases: a release for parents reaches an account whose child's
// relationship grants parent_portal.access at the school or group it names;
// another group, another school, a release for schools, a revoked release,
// a deleted document and a removed right reach nobody. A parent who already
// sees the document through an older active release hears nothing again; a
// new release after the revocation is a new event.
func TestReleaseNoticeFollowsTheRelationshipAndTheActiveRelease(t *testing.T) {
	t.Parallel()
	notices, pushes, db := traegerDocumentNotices(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	organization := organizationOf(t, db, chain.TenantID)
	testpkg.OwnTraegerDocuments(t, db, organization)
	var group, otherGroup int64
	require.NoError(t, db.NewRaw(`INSERT INTO education.groups (tenant_id, name) VALUES (?, 'Sonnenblumen') RETURNING id`, chain.TenantID).Scan(ctx, &group))
	require.NoError(t, db.NewRaw(`INSERT INTO education.groups (tenant_id, name) VALUES (?, 'Mondgruppe') RETURNING id`, chain.TenantID).Scan(ctx, &otherGroup))
	_, err := db.ExecContext(ctx, `UPDATE users.student_school_memberships SET group_id = ? WHERE tenant_id = ? AND student_profile_id = ?`, group, chain.TenantID, chain.StudentID)
	require.NoError(t, err)
	sibling := testpkg.CreateTestSchoolInOrganization(t, db, organization, "Andere OGS Push")
	own := sharedDocumentTraeger{t: t, db: db, organization: organization}

	letter := own.document("Elternbrief")
	forSchool := own.release(letter, "parents", chain.TenantID, nil)
	require.NoError(t, notices.NotifyTraegerDocumentRelease(ctx, organization, forSchool))
	require.Equal(t, []parentService.BillingDocumentPush{{
		Kind: parentService.DocumentPushRelease, TenantID: chain.TenantID, AccountID: chain.AccountID,
		StudentIDs: []int64{chain.StudentID}, DocumentID: forSchool,
	}}, pushes.to(chain.AccountID))

	trip := own.document("Ausflug")
	forGroup := own.release(trip, "parents", chain.TenantID, group)
	require.NoError(t, notices.NotifyTraegerDocumentRelease(ctx, organization, forGroup))
	require.Len(t, pushes.to(chain.AccountID), 2, "the child's group")

	silent := map[string]int64{
		"another group":        own.release(own.document("Mondgruppe"), "parents", chain.TenantID, otherGroup),
		"another school":       own.release(own.document("Andere Schule"), "parents", sibling, nil),
		"released for schools": own.release(own.document("Nur Schulen"), "schools", nil, nil),
		"already seen":         own.release(letter, "parents", chain.TenantID, group),
		"unknown release":      forSchool + 1_000_000,
	}
	revoked := own.release(own.document("Zurückgezogen"), "parents", chain.TenantID, nil)
	own.revoke(revoked)
	silent["revoked"] = revoked
	deletedDocument := own.document("Gelöscht")
	deleted := own.release(deletedDocument, "parents", nil, nil)
	_, err = db.ExecContext(ctx, `UPDATE platform.traeger_documents SET deleted_at = NOW(), content = NULL WHERE id = ?`, deletedDocument)
	require.NoError(t, err)
	silent["deleted document"] = deleted
	for name, release := range silent {
		require.NoError(t, notices.NotifyTraegerDocumentRelease(ctx, organization, release), name)
		assert.Len(t, pushes.to(chain.AccountID), 2, name)
	}
	foreign := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Fremder Träger Push", Slug: fmt.Sprintf("traeger-push-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerDocuments(t, db, foreign)
	require.NoError(t, notices.NotifyTraegerDocumentRelease(ctx, foreign, forSchool), "the release under another Träger")
	assert.Len(t, pushes.to(chain.AccountID), 2)

	// A new release after the revocation of the only one is a new document
	// in the inbox again.
	own.revoke(forSchool)
	own.revoke(silent["already seen"])
	again := own.release(letter, "parents", chain.TenantID, nil)
	require.NoError(t, notices.NotifyTraegerDocumentRelease(ctx, organization, again))
	require.Len(t, pushes.to(chain.AccountID), 3)
	assert.Equal(t, again, pushes.to(chain.AccountID)[2].DocumentID)

	// The relationship decides at the send: without parent_portal.access
	// nothing reaches the account.
	_, err = db.ExecContext(ctx, `UPDATE auth.guardian_student_access SET permissions = '{"parent_portal.pickup.manage": true}'::jsonb
		WHERE relationship_id IN (SELECT id FROM users.student_guardian_relationships WHERE guardian_profile_id = ?)`, chain.GuardianProfileID)
	require.NoError(t, err)
	late := own.release(own.document("Nach Rechteentzug"), "parents", chain.TenantID, nil)
	require.NoError(t, notices.NotifyTraegerDocumentRelease(ctx, organization, late))
	assert.Len(t, pushes.to(chain.AccountID), 3, "a removed right stops the push")
}
