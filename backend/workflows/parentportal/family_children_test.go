package parentportal_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

// Parents read and state the children of their family only on a contract
// sent to their own address (#5918): the inbox right reads, the right
// parent_portal.enrollment.submit states; another family's contract is not
// found; the number keys to the account's address, and every read and
// statement lands in the trail with the account. A stated number waits for
// the Abrechnung and does not show as confirmed.
func TestParentsStateTheFamilyChildrenOnlyForTheirOwnFamily(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	portal := parentportalcompose.New(documentInboxDependencies(t, db, &owners.CareContractTestMails{}))
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, chain.TenantID).Scan(ctx, &organization))
	testpkg.OwnTraegerCareContracts(t, db, organization)
	own := activeInboxContract(t, db, organization, chain.TenantID, chain.StudentID, "Felix", chain.Email)
	otherFamily := activeInboxContract(t, db, organization, chain.TenantID, chain.StudentID, "Tom", "andere.familie@example.org")

	_, err := portal.StateFamilyChildren(ctx, chain.AccountID, organization, otherFamily, 3)
	assert.ErrorIs(t, err, parentService.ErrBillingDocumentNotFound, "another family's contract")
	_, err = portal.FamilyChildren(ctx, chain.AccountID, organization, otherFamily)
	assert.ErrorIs(t, err, parentService.ErrBillingDocumentNotFound)

	var link int64
	require.NoError(t, db.NewRaw(`SELECT id FROM users.student_guardian_relationships WHERE student_id = ? AND guardian_profile_id = ?`,
		chain.StudentID, chain.GuardianProfileID).Scan(ctx, &link))
	require.NoError(t, testpkg.SetTestStudentGuardianPermissions(ctx, db, link, map[string]interface{}{
		"parent_portal.access": true, "parent_portal.enrollments.view": true,
	}))
	read, err := portal.FamilyChildren(ctx, chain.AccountID, organization, own)
	require.NoError(t, err, "the inbox right reads")
	assert.False(t, read.CanState, "but does not state")
	assert.Nil(t, read.StatedChildren, "nothing stated yet")
	_, err = portal.StateFamilyChildren(ctx, chain.AccountID, organization, own, 3)
	require.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied, "reading the inbox does not state the number")

	require.NoError(t, testpkg.SetTestStudentGuardianPermissions(ctx, db, link, map[string]interface{}{
		"parent_portal.access": true, "parent_portal.enrollments.view": true, "parent_portal.enrollment.submit": true,
	}))
	_, err = portal.StateFamilyChildren(ctx, chain.AccountID, organization, own, 0)
	require.ErrorIs(t, err, parentService.ErrFamilyChildrenInvalid, "no child is no number")
	_, err = portal.StateFamilyChildren(ctx, chain.AccountID, organization, own, 21)
	require.ErrorIs(t, err, parentService.ErrFamilyChildrenInvalid)
	stated, err := portal.StateFamilyChildren(ctx, chain.AccountID, organization, own, 3)
	require.NoError(t, err)
	assert.True(t, stated.CanState)
	require.NotNil(t, stated.StatedChildren)
	assert.Equal(t, 3, *stated.StatedChildren)
	assert.Nil(t, stated.ConfirmedChildren, "the statement waits for the Abrechnung")

	var family string
	require.NoError(t, db.NewRaw(`SELECT family_email FROM parent_billing.family_child_counts WHERE organization_id = ?`, organization).
		Scan(ctx, &family))
	assert.Equal(t, strings.ToLower(strings.TrimSpace(chain.Email)), family, "the family is the account's own address")
	var trail []string
	require.NoError(t, db.NewRaw(`SELECT action FROM parent_billing.family_child_count_events
		WHERE organization_id = ? AND care_contract_id = ? AND actor = 'parent' AND actor_account_id = ? ORDER BY id`,
		organization, own, chain.AccountID).Scan(ctx, &trail))
	assert.Equal(t, []string{"read", "state"}, trail, "the read and the statement, both with the account")
}
