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

// sharedDocumentInbox is the parents portal over the real Organisation &
// Tenancy, People Directory and Identity & Access.
func sharedDocumentInbox(t *testing.T) (*parentService.Portal, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	people := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	organizations, err := owners.NewOrganizationTenancy(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	identity, err := api.NewIdentityAccessForTests(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	return parentportalcompose.New(parentportalcompose.Dependencies{
		Memberships:         parentMemberships(t, db),
		StudentGuardianRepo: guardStudentGuardians(t, db),
		GuardianChanges:     owners.ParentGuardianChanges(), DeparturePolicy: owners.ParentDeparturePolicy(),
		GuardianPolicy: owners.NewParentGuardianPolicy(), Fingerprints: owners.SecurityFingerprints{},
		Declarations: owners.ParentDeclarationPolicy(), Locales: owners.ParentPortalLocales(), Runtime: owners.ParentPortalRuntime{},
		ChildRepo: repos.ParentChildReads(), PersonRepo: owners.NewParentPersonReads(people.Person),
		GuardianProfileRepo: owners.NewParentGuardianProfileReads(people.GuardianProfile),
		StudentRepo:         owners.NewParentStudentReads(people.Student),
		Logger:              slog.Default(), CarePlanPolicy: owners.NewParentCarePlanPolicy(),
		BillingDocuments: owners.NewParentBillingDocuments(), ContractDocuments: owners.NewParentContractDocuments(owners.NewParentContractSigningForTests(&owners.CareContractTestMails{}), owners.CareContractTestHolidays{}),
		SharedDocuments: owners.NewParentSharedDocuments(), TraegerRuntime: owners.ParentPortalRuntime{},
		SchoolTraeger: owners.NewParentSchoolTraeger(organizations), ParentAccounts: owners.NewParentAccounts(identity),
	}), db
}

// sharedDocumentTraeger writes the documents and releases of one Träger as
// Organisation & Tenancy stores them.
type sharedDocumentTraeger struct {
	t            *testing.T
	db           *bun.DB
	organization int64
}

func (f sharedDocumentTraeger) document(title string) int64 {
	f.t.Helper()
	var id int64
	require.NoError(f.t, f.db.NewRaw(`INSERT INTO platform.traeger_documents (organization_id, title, file_name, content_type, size_bytes, content)
		VALUES (?, ?, 'brief.pdf', 'application/pdf', 8, '%PDF-1.4'::bytea) RETURNING id`, f.organization, title).Scan(context.Background(), &id))
	return id
}

// release releases the document; school and group may be nil.
func (f sharedDocumentTraeger) release(document int64, audience string, school, group any) int64 {
	f.t.Helper()
	var id int64
	name := any(nil)
	if group != nil {
		name = "Sonnenblumen"
	}
	require.NoError(f.t, f.db.NewRaw(`INSERT INTO platform.traeger_document_releases (organization_id, document_id, audience, school_id, group_id, group_name)
		VALUES (?, ?, ?, ?, ?, ?) RETURNING id`, f.organization, document, audience, school, group, name).Scan(context.Background(), &id))
	return id
}

func (f sharedDocumentTraeger) revoke(release int64) {
	f.t.Helper()
	_, err := f.db.ExecContext(context.Background(), `UPDATE platform.traeger_document_releases SET revoked_at = NOW() WHERE id = ?`, release)
	require.NoError(f.t, err)
}

func (f sharedDocumentTraeger) reads(accountID int64) int {
	f.t.Helper()
	var n int
	require.NoError(f.t, f.db.NewRaw(`SELECT COUNT(*) FROM platform.traeger_document_reads
		WHERE organization_id = ? AND account_id = ? AND reader = 'parent'`, f.organization, accountID).Scan(context.Background(), &n))
	return n
}

func sharedTitles(documents []parentService.SharedDocument) []string {
	titles := make([]string, 0, len(documents))
	for _, document := range documents {
		titles = append(titles, document.Title)
	}
	return titles
}

// TestSharedDocumentsFollowTheRelationship pins #4692 for parents: a parent
// sees a document released for the parents of all schools of the Träger, of
// the child's school or of the child's group, never one for another group,
// another school, the schools only or another Träger. The relationship
// decides: without parent_portal.access nothing opens. A revoked release ends
// the access at once; every opening is recorded with the account.
func TestSharedDocumentsFollowTheRelationship(t *testing.T) {
	t.Parallel()
	portal, db := sharedDocumentInbox(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, chain.TenantID).Scan(ctx, &organization))
	testpkg.OwnTraegerDocuments(t, db, organization)
	var group, otherGroup int64
	require.NoError(t, db.NewRaw(`INSERT INTO education.groups (tenant_id, name) VALUES (?, 'Sonnenblumen') RETURNING id`, chain.TenantID).Scan(ctx, &group))
	require.NoError(t, db.NewRaw(`INSERT INTO education.groups (tenant_id, name) VALUES (?, 'Mondgruppe') RETURNING id`, chain.TenantID).Scan(ctx, &otherGroup))
	moved, err := db.ExecContext(ctx, `UPDATE users.student_school_memberships SET group_id = ? WHERE tenant_id = ? AND student_profile_id = ?`, group, chain.TenantID, chain.StudentID)
	require.NoError(t, err)
	rows, err := moved.RowsAffected()
	require.NoError(t, err)
	require.EqualValues(t, 1, rows, "the child's membership joins the group")
	sibling := testpkg.CreateTestSchoolInOrganization(t, db, organization, "Andere OGS Dokumente")

	own := sharedDocumentTraeger{t: t, db: db, organization: organization}
	forAll := own.document("Hausordnung")
	own.release(forAll, "parents", nil, nil)
	forSchool := own.document("Elternabend")
	schoolRelease := own.release(forSchool, "parents", chain.TenantID, nil)
	forGroup := own.document("Ausflug Sonnenblumen")
	own.release(forGroup, "parents", chain.TenantID, group)
	forOtherGroup := own.document("Ausflug Mondgruppe")
	own.release(forOtherGroup, "parents", chain.TenantID, otherGroup)
	forSibling := own.document("Andere Schule")
	own.release(forSibling, "parents", sibling, nil)
	forSchools := own.document("Nur für Schulen")
	own.release(forSchools, "schools", nil, nil)
	unreleased := own.document("Entwurf")

	foreignOrganization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Fremder Träger Dokumente", Slug: fmt.Sprintf("traeger-dokumente-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerDocuments(t, db, foreignOrganization)
	foreign := sharedDocumentTraeger{t: t, db: db, organization: foreignOrganization}
	foreignDocument := foreign.document("Fremd")
	foreign.release(foreignDocument, "parents", nil, nil)

	documents, err := portal.ListSharedDocuments(ctx, chain.AccountID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"Hausordnung", "Elternabend", "Ausflug Sonnenblumen"}, sharedTitles(documents))
	for _, document := range documents {
		assert.Equal(t, organization, document.OrganizationID)
		assert.NotEmpty(t, document.TraegerName)
	}

	file, err := portal.SharedDocumentFile(ctx, chain.AccountID, organization, forGroup)
	require.NoError(t, err)
	assert.Equal(t, []byte("%PDF-1.4"), file.Data)
	assert.Equal(t, "application/pdf", file.ContentType)
	assert.Equal(t, 1, own.reads(chain.AccountID), "the opening is recorded with the account")

	for name, request := range map[string]struct{ organization, document int64 }{
		"another group":          {organization, forOtherGroup},
		"another school":         {organization, forSibling},
		"released for schools":   {organization, forSchools},
		"not released":           {organization, unreleased},
		"another Träger":         {foreignOrganization, foreignDocument},
		"document under another": {foreignOrganization, forAll},
	} {
		_, err := portal.SharedDocumentFile(ctx, chain.AccountID, request.organization, request.document)
		assert.ErrorIs(t, err, parentService.ErrSharedDocumentNotFound, name)
	}
	assert.Equal(t, 1, own.reads(chain.AccountID), "a refused opening is not recorded")
	assert.Zero(t, foreign.reads(chain.AccountID))

	own.revoke(schoolRelease)
	documents, err = portal.ListSharedDocuments(ctx, chain.AccountID)
	require.NoError(t, err)
	assert.NotContains(t, sharedTitles(documents), "Elternabend", "revoking ends the access at once")
	_, err = portal.SharedDocumentFile(ctx, chain.AccountID, organization, forSchool)
	assert.ErrorIs(t, err, parentService.ErrSharedDocumentNotFound)

	// The relationship decides: without parent_portal.access the same
	// guardian link opens nothing.
	_, err = db.ExecContext(ctx, `UPDATE auth.guardian_student_access SET permissions = '{"parent_portal.pickup.manage": true}'::jsonb
		WHERE relationship_id IN (SELECT id FROM users.student_guardian_relationships WHERE guardian_profile_id = ?)`,
		chain.GuardianProfileID)
	require.NoError(t, err)
	_, err = portal.ListSharedDocuments(ctx, chain.AccountID)
	assert.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied)
	_, err = portal.SharedDocumentFile(ctx, chain.AccountID, organization, forAll)
	assert.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied)
	assert.Equal(t, 1, own.reads(chain.AccountID))
}
