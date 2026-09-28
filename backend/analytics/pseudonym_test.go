package analytics

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sha256OfSchool42Account7 is the SHA-256 of "moto-analytics:v1:42:7", the
// digest the Security Runtime fingerprint returns for that input. The browser
// pins the same vector (frontend/src/lib/analytics-pseudonym.test.ts); a
// mismatch splits one person into two profiles.
const sha256OfSchool42Account7 = "4b9630678fc1afce76ce690721aaf9494e35adb17924625345a35553831df2ba"

type recordingFingerprint struct{ inputs []string }

func (r *recordingFingerprint) hash(content []byte) string {
	r.inputs = append(r.inputs, string(content))
	if string(content) == "moto-analytics:v1:42:7" {
		return sha256OfSchool42Account7
	}
	return "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
}

func TestPseudonymousIDHashesSchoolAndAccount(t *testing.T) {
	t.Parallel()

	fingerprint := &recordingFingerprint{}

	assert.Equal(t, "pseudo_4b9630678fc1afce76ce690721aaf949", pseudonymousID(fingerprint.hash, 42, 7))
	assert.Equal(t, []string{"moto-analytics:v1:42:7"}, fingerprint.inputs,
		"the browser hashes exactly this input")
}

func TestPseudonymousIDNeedsAFingerprintAndBothIDs(t *testing.T) {
	t.Parallel()

	fingerprint := &recordingFingerprint{}
	assert.Empty(t, pseudonymousID(nil, 42, 7))
	assert.Empty(t, pseudonymousID(fingerprint.hash, 0, 7))
	assert.Empty(t, pseudonymousID(fingerprint.hash, 42, 0))
	assert.Empty(t, pseudonymousID(func([]byte) string { return "short" }, 42, 7))
}

// Only an event the middleware marked as a person goes out under the
// pseudonymous ID with a person profile, and only with a fingerprint.
func TestCaptureSendsAMarkedPersonUnderItsPseudonym(t *testing.T) {
	t.Parallel()

	server := newPostHogServer(t, http.StatusOK)
	tracker, err := New("phc_test", server.URL, testDeployment, nil, WithFingerprint((&recordingFingerprint{}).hash))
	require.NoError(t, err)

	tracker.CaptureContext(withPerson(context.Background(), 42, 7), "school:42", "group_created", nil)
	tracker.CaptureContext(context.Background(), "school:42", "group_created", map[string]any{
		"$process_person_profile": true,
	})
	require.NoError(t, tracker.Close())

	events := server.events()
	require.Len(t, events, 2)
	assert.Equal(t, "pseudo_4b9630678fc1afce76ce690721aaf949", events[0].DistinctID)
	assert.Equal(t, true, events[0].Properties["$process_person_profile"])
	assert.NotContains(t, events[0].Properties, "account_id")
	assert.Equal(t, "school:42", events[1].DistinctID)
	assert.Equal(t, false, events[1].Properties["$process_person_profile"], "a caller cannot turn a person on")
}

func TestCaptureWithoutFingerprintKeepsAPersonAnonymous(t *testing.T) {
	t.Parallel()

	server := newPostHogServer(t, http.StatusOK)
	tracker := newServerTracker(t, server, nil)

	tracker.CaptureContext(withPerson(context.Background(), 42, 7), "school:42", "group_created", nil)
	require.NoError(t, tracker.Close())

	events := server.events()
	require.Len(t, events, 1)
	assert.Equal(t, "school:42", events[0].DistinctID)
	assert.Equal(t, false, events[0].Properties["$process_person_profile"])
}

func newFreigabeRouter(tracker Tracker, actor Actor, freigabe map[int64]bool) http.Handler {
	root := chi.NewRouter()
	root.Use(CoreActionMiddleware(CoreActionConfig{
		Tracker:      tracker,
		RequestActor: func(*http.Request) (Actor, bool) { return actor, true },
		AnalyseFreigabe: func(_ context.Context, schoolID int64) bool {
			return freigabe[schoolID]
		},
	}))
	root.Route("/api", func(r chi.Router) {
		groups := chi.NewRouter()
		groups.Post("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) })
		r.Mount("/groups", groups)
	})
	return root
}

func TestCoreActionMiddlewareMarksAPersonOnlyForOGSWithAnalyseFreigabe(t *testing.T) {
	t.Parallel()

	ogsStaff := Actor{Surface: SurfaceOGS, Role: RoleStaff, SchoolID: 42, AccountID: 7}
	cases := []struct {
		name     string
		actor    Actor
		freigabe map[int64]bool
		distinct string
		person   bool
	}{
		{"OGS with Freigabe", ogsStaff, map[int64]bool{42: true}, "school:42", true},
		{"OGS without Freigabe", ogsStaff, map[int64]bool{}, "school:42", false},
		{"Freigabe of another school", ogsStaff, map[int64]bool{43: true}, "school:42", false},
		{"OGS session without account", Actor{Surface: SurfaceOGS, Role: RoleStaff, SchoolID: 42}, map[int64]bool{42: true}, "school:42", false},
		{"school portal", Actor{Surface: SurfaceSchool, Role: RoleLehrkraft, SchoolID: 42, AccountID: 7}, map[int64]bool{42: true}, "school:42", false},
		{"parents portal", Actor{Surface: SurfaceParents, Role: RoleGuardian, AccountID: 7}, map[int64]bool{42: true}, "surface:parents", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tracker := &personRecordingTracker{}
			rec := serve(newFreigabeRouter(tracker, tc.actor, tc.freigabe), http.MethodPost, "/api/groups/", "{}", nil)
			require.Equal(t, http.StatusCreated, rec.Code)

			require.Len(t, tracker.events, 1)
			event := tracker.events[0]
			assert.Equal(t, tc.distinct, event.distinctID)
			assert.Equal(t, tc.person, event.person)
			if tc.person {
				assert.Equal(t, person{schoolID: 42, accountID: 7}, event.who)
			}
			assert.NotContains(t, event.props, "account_id")
		})
	}
}

// personRecordingTracker records the person the middleware marked, which
// the real tracker turns into the pseudonymous ID.
type personRecordingTracker struct {
	events []personEvent
}

type personEvent struct {
	distinctID string
	person     bool
	who        person
	props      map[string]any
}

func (r *personRecordingTracker) Capture(distinctID, event string, props map[string]any) {
	r.CaptureContext(context.Background(), distinctID, event, props)
}

func (r *personRecordingTracker) CaptureContext(ctx context.Context, distinctID, _ string, props map[string]any) {
	who, ok := personFromContext(ctx)
	r.events = append(r.events, personEvent{distinctID: distinctID, person: ok, who: who, props: props})
}

func (*personRecordingTracker) Close() error { return nil }
