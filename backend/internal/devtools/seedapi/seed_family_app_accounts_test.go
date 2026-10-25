package api

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func demoGuardianIDs() map[string]int64 {
	ids := make(map[string]int64, len(DemoGuardians))
	for index, guardian := range DemoGuardians {
		ids[guardian.FirstName+" "+guardian.LastName] = int64(1000 + index)
	}
	return ids
}

// Every demo child gets a parent in the app (#3894), except the families the
// demo keeps without one on purpose and the ones that already have an account.
func TestFamilyAppGuardiansCoverEveryOtherChildOnce(t *testing.T) {
	t.Parallel()

	ids := demoGuardianIDs()
	// Sabine Schneider (child 0) already has an account; Klaus Schneider is the
	// other parent of the same child.
	parents := []ParentCredentials{{GuardianID: ids["Sabine Schneider"]}}

	indexes, err := familyAppGuardianIndexes(ids, parents)
	require.NoError(t, err)

	// A guardian of siblings has one entry per child and one account for all.
	entriesByID := make(map[int64][]DemoGuardian, len(DemoGuardians))
	for _, guardian := range DemoGuardians {
		id := ids[guardian.FirstName+" "+guardian.LastName]
		entriesByID[id] = append(entriesByID[id], guardian)
	}
	children := map[int]bool{}
	listed := map[int64]bool{}
	for _, index := range indexes {
		require.Less(t, index, len(DemoGuardians), "index %d is a demo guardian", index)
		id := ids[demoGuardianKey(DemoGuardians[index])]
		assert.False(t, listed[id], "guardian %d is listed once", id)
		listed[id] = true
		for _, guardian := range entriesByID[id] {
			assert.True(t, guardian.IsPrimary, "%s %s", guardian.FirstName, guardian.LastName)
			assert.False(t, children[guardian.StudentIndex], "child %d gets one account", guardian.StudentIndex)
			children[guardian.StudentIndex] = true
		}
	}
	assert.False(t, children[0], "child 0 already has a parent in the app")
	for index := range familiesWithoutApp {
		assert.False(t, children[index], "child %d stays without app", index)
	}
	assert.Len(t, children, len(DemoStudents)-1-len(familiesWithoutApp))
}

func TestFamilyAppGuardiansReportAMissingGuardian(t *testing.T) {
	t.Parallel()

	_, err := familyAppGuardianIndexes(map[string]int64{}, nil)
	require.Error(t, err)
}

// The families without app must be children the re-enrollment overview lists:
// not the top grade, which leaves the school.
func TestFamiliesWithoutAppAreReEnrolled(t *testing.T) {
	t.Parallel()

	for index := range familiesWithoutApp {
		require.Less(t, index, len(DemoStudents))
		grade, err := demoClassGrade(DemoStudents[index].Class)
		require.NoError(t, err)
		assert.Less(t, grade, 4, "child %d", index)
	}
}

// The parents of approved online enrollments get an account, except two the
// school still sees waiting. The listing carries no token (#4902).
func TestEnrollmentInvitationsKeepTwoOpen(t *testing.T) {
	t.Parallel()

	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		assert.Equal(t, "/api/guardians/invitations/pending", r.URL.Path)
		_, _ = fmt.Fprint(w, `{"status":"success","data":[`+
			`{"id":1,"guardian_profile_id":704},{"id":2,"guardian_profile_id":701},`+
			`{"id":3,"guardian_profile_id":703},{"id":4,"guardian_profile_id":702}]}`)
	})
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false)}
	invitations, err := enrollmentInvitationsToAccept(rt, AuthRef{Token: "admin"})
	require.NoError(t, err)
	assert.Equal(t, []enrollmentInvitation{
		{ID: 3, GuardianProfileID: 703},
		{ID: 1, GuardianProfileID: 704},
	}, invitations)
}
