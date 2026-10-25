package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Siblings share their parents: the seed creates one profile per person and
// links it to every child, with the same role on each link.
func TestSeedGuardiansLinksSiblingsToOneProfile(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	created := 0
	links := map[int64][]map[string]any{}
	server := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path == "/api/guardians" {
			created++
			_, _ = fmt.Fprintf(w, `{"status":"success","data":{"id":%d}}`, created)
			return
		}
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		studentID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/guardians/students/"), "/guardians")
		var id int64
		_, err := fmt.Sscan(studentID, &id)
		require.NoError(t, err)
		links[id] = append(links[id], body)
		_, _ = fmt.Fprint(w, `{"status":"success"}`)
	})
	defer server.Close()

	fs := NewFixedSeeder(newTestClient(server.URL, false), false, "")
	for index := range DemoStudents {
		fs.studentIDByIndex[index] = int64(index + 100)
	}
	result := &FixedResult{}
	require.NoError(t, fs.seedGuardians(context.Background(), result))

	assert.Equal(t, demoGuardianProfileCount(), created)
	assert.Less(t, created, len(DemoGuardians), "the demo has siblings")
	assert.Equal(t, len(DemoGuardians), result.GuardianCount)

	// Aida (3) and Baran (4) Amini: Shirin and Reza are one profile each.
	aida, baran := links[103], links[104]
	require.Len(t, aida, 2)
	require.Len(t, baran, 2)
	for i := range aida {
		assert.Equal(t, aida[i]["guardian_profile_id"], baran[i]["guardian_profile_id"])
		assert.Equal(t, aida[i]["guardian_role"], baran[i]["guardian_role"])
	}
}

// A parent with an app account reaches all of their children, the child of
// their own entry first.
func TestDemoGuardianStudentIDsIncludeSiblings(t *testing.T) {
	t.Parallel()

	shirin := parentAccountGuardianIndexes[2]
	require.Equal(t, "Shirin Amini", demoGuardianKey(DemoGuardians[shirin]))
	studentIDs := map[int]int64{3: 103, 4: 104}

	ids, err := demoGuardianStudentIDs(studentIDs, shirin)
	require.NoError(t, err)
	assert.Equal(t, []int64{104, 103}, ids)

	_, err = demoGuardianStudentIDs(map[int]int64{4: 104}, shirin)
	require.Error(t, err)
	assert.NotContains(t, sickDemoCandidates(), 3, "Aida has a parent in the app")
}

// The scripted parents keep the full role their parent-portal flows need.
func TestDemoGuardianLinkKeepsAppParentsFullRole(t *testing.T) {
	t.Parallel()

	for _, index := range parentAccountGuardianIndexes {
		link := demoGuardianLink(1, DemoGuardians[index])
		assert.NotContains(t, link, "guardian_role", demoGuardianKey(DemoGuardians[index]))
		assert.Equal(t, true, link["can_pickup"])
	}
}
