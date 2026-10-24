package analytics

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// consentRecordingTracker records what links an event to a browser: the
// session and the pseudonymous person.
type consentRecordingTracker struct {
	events []consentEvent
}

type consentEvent struct {
	sessionID string
	person    bool
	distinct  string
}

func (r *consentRecordingTracker) Capture(distinctID, event string, props map[string]any) {
	r.CaptureContext(context.Background(), distinctID, event, props)
}

func (r *consentRecordingTracker) CaptureContext(ctx context.Context, distinctID, _ string, _ map[string]any) {
	_, person := personFromContext(ctx)
	r.events = append(r.events, consentEvent{sessionID: SessionIDFromContext(ctx), person: person, distinct: distinctID})
}

func (*consentRecordingTracker) Close() error { return nil }

func newConsentRouter(tracker Tracker, actor Actor, demo bool, consent func(context.Context, int64) bool) http.Handler {
	root := chi.NewRouter()
	root.Use(CoreActionMiddleware(CoreActionConfig{
		Policy: CollectionPolicy{
			Demo:          demo,
			SchoolEnabled: func(context.Context, int64) (bool, error) { return true, nil },
			ParentSchools: func(_ context.Context, id int64) (map[int64][]int64, error) {
				return map[int64][]int64{id: {42}}, nil
			},
		},
		Tracker:         tracker,
		RequestActor:    func(*http.Request) (Actor, bool) { return actor, true },
		AnalyseFreigabe: func(context.Context, int64) bool { return true },
		Consent:         consent,
	}))
	root.Route("/api", func(r chi.Router) {
		groups := chi.NewRouter()
		groups.Post("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) })
		r.Mount("/groups", groups)
	})
	return root
}

// Without the account's consent (§ 25 TDDDG, #4901) a core action still
// counts for its school, but it names neither the browser session nor the
// pseudonymous person. Unknown answers and failed lookups count as no consent.
func TestCoreActionMiddlewareLinksTheBrowserOnlyWithConsent(t *testing.T) {
	t.Parallel()

	ogsStaff := Actor{Surface: SurfaceOGS, Role: RoleStaff, SchoolID: 42, AccountID: 7}
	granted := func(_ context.Context, accountID int64) bool { return accountID == 7 }
	refused := func(context.Context, int64) bool { return false }
	cases := []struct {
		name    string
		actor   Actor
		demo    bool
		consent func(context.Context, int64) bool
		session string
		person  bool
	}{
		{"OGS with consent", ogsStaff, false, granted, testSessionID, true},
		{"OGS without consent", ogsStaff, false, refused, "", false},
		{"no consent lookup wired", ogsStaff, false, nil, "", false},
		{"consent of another account", Actor{Surface: SurfaceOGS, Role: RoleStaff, SchoolID: 42, AccountID: 8}, false, granted, "", false},
		{"session without account", Actor{Surface: SurfaceOGS, Role: RoleAdmin, SchoolID: 42}, false, granted, "", false},
		{"school portal with consent", Actor{Surface: SurfaceSchool, Role: RoleLehrkraft, SchoolID: 42, AccountID: 7}, false, granted, testSessionID, false},
		{"school portal without consent", Actor{Surface: SurfaceSchool, Role: RoleLehrkraft, SchoolID: 42, AccountID: 7}, false, refused, "", false},
		{"parents with consent", Actor{Surface: SurfaceParents, Role: RoleGuardian, AccountID: 7}, false, granted, testSessionID, false},
		{"parents without consent", Actor{Surface: SurfaceParents, Role: RoleGuardian, AccountID: 7}, false, refused, "", false},
		{"public demo needs no consent", ogsStaff, true, refused, testSessionID, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tracker := &consentRecordingTracker{}
			rec := serve(newConsentRouter(tracker, tc.actor, tc.demo, tc.consent), http.MethodPost, "/api/groups/", "{}",
				map[string]string{SessionIDHeader: testSessionID})
			require.Equal(t, http.StatusCreated, rec.Code)

			require.Len(t, tracker.events, 1, "the action still counts")
			assert.Equal(t, tc.session, tracker.events[0].sessionID)
			assert.Equal(t, tc.person, tracker.events[0].person)
		})
	}
}
