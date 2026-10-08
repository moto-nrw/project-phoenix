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

	guardians, err := familyAppGuardians(ids, parents)
	require.NoError(t, err)

	byID := make(map[int64]DemoGuardian, len(DemoGuardians))
	for _, guardian := range DemoGuardians {
		byID[ids[guardian.FirstName+" "+guardian.LastName]] = guardian
	}
	children := map[int]bool{}
	for _, id := range guardians {
		guardian, ok := byID[id]
		require.True(t, ok, "guardian %d is a demo guardian", id)
		assert.True(t, guardian.IsPrimary, "%s %s", guardian.FirstName, guardian.LastName)
		assert.False(t, children[guardian.StudentIndex], "child %d gets one account", guardian.StudentIndex)
		children[guardian.StudentIndex] = true
	}
	assert.False(t, children[0], "child 0 already has a parent in the app")
	for index := range familiesWithoutApp {
		assert.False(t, children[index], "child %d stays without app", index)
	}
	assert.Len(t, children, len(DemoStudents)-1-len(familiesWithoutApp))
}

func TestFamilyAppGuardiansReportAMissingGuardian(t *testing.T) {
	t.Parallel()

	_, err := familyAppGuardians(map[string]int64{}, nil)
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

// The parents of approved online enrollments accept their existing invitation,
// except two the school still sees waiting.
func TestEnrollmentInvitationsKeepTwoOpen(t *testing.T) {
	t.Parallel()

	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		assert.Equal(t, "/api/guardians/invitations/pending", r.URL.Path)
		assert.Equal(t, "true", r.Header.Get(seedTokenHeader))
		_, _ = fmt.Fprint(w, `{"status":"success","data":[`+
			`{"id":1,"guardian_profile_id":704,"token":"invite-704"},{"id":2,"guardian_profile_id":701,"token":"invite-701"},`+
			`{"id":3,"guardian_profile_id":703,"token":"invite-703"},{"id":4,"guardian_profile_id":702,"token":"invite-702"}]}`)
	})
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false)}
	invitations, err := enrollmentInvitationsToAccept(rt, AuthRef{Token: "admin"})
	require.NoError(t, err)
	assert.Equal(t, []enrollmentInvitation{
		{ID: 3, GuardianProfileID: 703, Token: "invite-703"},
		{ID: 1, GuardianProfileID: 704, Token: "invite-704"},
	}, invitations)
}
