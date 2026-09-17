package domain

import (
	"testing"
)

// Ported from services/platform (#3252): the listing keeps the owner's
// organisation name order, equal names share a position, and the incoming
// school order survives within one organisation.
func TestSortTenantAccessByOrganizationPreservesDatabaseNameOrdering(t *testing.T) {
	t.Parallel()

	entries := []AccountTenantAccess{
		{OrganizationID: 2, SchoolName: "A"},
		{OrganizationID: 3, SchoolName: "B"},
		{OrganizationID: 1, SchoolName: "C"},
	}
	organizations := []OrganizationName{{ID: 3, Name: "Alpha"}, {ID: 1, Name: "Same"}, {ID: 2, Name: "Same"}}

	if !SortTenantAccessByOrganization(entries, organizations) {
		t.Fatal("every organisation is known")
	}
	gotIDs := [3]int64{entries[0].OrganizationID, entries[1].OrganizationID, entries[2].OrganizationID}
	if gotIDs != [3]int64{3, 2, 1} {
		t.Fatalf("organization order = %v, want [3 2 1]", gotIDs)
	}
	for index, want := range []string{"Alpha", "Same", "Same"} {
		if entries[index].OrganizationName != want {
			t.Fatalf("entry %d organisation name = %q, want %q", index, entries[index].OrganizationName, want)
		}
	}
}

func TestSortTenantAccessByOrganizationReportsUnknownOrganization(t *testing.T) {
	t.Parallel()

	entries := []AccountTenantAccess{{OrganizationID: 9}}
	if SortTenantAccessByOrganization(entries, []OrganizationName{{ID: 1, Name: "Known"}}) {
		t.Fatal("an organisation missing from the owner's answer must be reported")
	}
}

func TestUnambiguousPersonIdentity(t *testing.T) {
	t.Parallel()

	t.Run("no candidates", func(t *testing.T) {
		t.Parallel()
		if _, found := UnambiguousPersonIdentity(nil); found {
			t.Fatal("no person means no identity")
		}
	})
	t.Run("students and deleted rows never qualify", func(t *testing.T) {
		t.Parallel()
		persons := []AccountPersonIdentity{
			{ID: 1, TenantID: 10, FirstName: "Kind", LastName: "Datensatz", IsStudent: true},
			{ID: 2, TenantID: 11, FirstName: "Alt", LastName: "Person", Deleted: true},
		}
		if _, found := UnambiguousPersonIdentity(persons); found {
			t.Fatal("a child's record or a deleted person is not the account holder's identity")
		}
	})
	t.Run("disagreeing names are refused", func(t *testing.T) {
		t.Parallel()
		persons := []AccountPersonIdentity{
			{ID: 1, TenantID: 10, FirstName: "Anna", LastName: "Beispiel"},
			{ID: 2, TenantID: 11, FirstName: "Bea", LastName: "Beispiel"},
		}
		if _, found := UnambiguousPersonIdentity(persons); found {
			t.Fatal("two spellings are an ambiguity the flow may not resolve")
		}
	})
	t.Run("agreeing names pick the stable representative", func(t *testing.T) {
		t.Parallel()
		persons := []AccountPersonIdentity{
			{ID: 7, TenantID: 12, FirstName: "Anna", LastName: "Beispiel"},
			{ID: 3, TenantID: 10, FirstName: " Anna ", LastName: "Beispiel "},
			{ID: 5, TenantID: 10, FirstName: "Anna", LastName: "Beispiel"},
		}
		selected, found := UnambiguousPersonIdentity(persons)
		if !found {
			t.Fatal("agreeing names are one identity")
		}
		if selected.ID != 3 {
			t.Fatalf("selected person %d, want the lowest tenant and id (3)", selected.ID)
		}
	})
}

func TestRoleOwnershipRules(t *testing.T) {
	t.Parallel()

	guardianTier := AccountTenantRole{ID: 1, Name: "custom", BaseRole: new("guardian")}
	if !RoleOwnedByOtherFeature(guardianTier) || !RoleBlocksAccessRevocation(guardianTier) {
		t.Fatal("guardian-tier roles belong to the parent portal relationship")
	}
	for _, name := range []string{"guardian", "user", "teacher"} {
		system := AccountTenantRole{ID: 2, Name: name, IsSystem: true}
		if !RoleOwnedByOtherFeature(system) || !RoleBlocksAccessRevocation(system) {
			t.Fatalf("the %s system role is managed by its own flow", name)
		}
	}
	customUser := AccountTenantRole{ID: 3, Name: "user", IsSystem: false}
	if RoleOwnedByOtherFeature(customUser) || RoleBlocksAccessRevocation(customUser) {
		t.Fatal("a school's own role named user is an ordinary staff role")
	}
	admin := AccountTenantRole{ID: 4, Name: "admin", IsSystem: true}
	if RoleOwnedByOtherFeature(admin) || RoleBlocksAccessRevocation(admin) {
		t.Fatal("the admin role is exchanged and revoked by the access flows")
	}
}
