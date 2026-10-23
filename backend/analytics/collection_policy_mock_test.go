package analytics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchoolCollectionGatesEveryCapturePath(t *testing.T) {
	t.Parallel()
	for _, state := range []struct {
		name    string
		enabled bool
		err     error
		want    int
	}{
		{"on", true, nil, 1}, {"off", false, nil, 0}, {"read error", true, errors.New("unavailable"), 0},
	} {
		t.Run(state.name, func(t *testing.T) {
			policy := CollectionPolicy{SchoolEnabled: func(context.Context, int64) (bool, error) { return state.enabled, state.err }}
			for _, event := range []string{"student_checked_in", "student_checked_out", "room_transfer", groupIdentifyEvent} {
				target := &recordingTracker{}
				tracker := GateSchoolTracker(target, policy.Demo, policy.SchoolEnabled)
				props := map[string]any{"school_id": int64(42)}
				if event == groupIdentifyEvent {
					props = map[string]any{"$group_key": "42", "$group_type": "school"}
				}
				tracker.Capture("school:42", event, props)
				assert.Len(t, target.captured(), state.want, event)
			}
			target := &recordingTracker{}
			router := chi.NewRouter()
			router.Use(CoreActionMiddleware(CoreActionConfig{Tracker: target, Policy: policy, RequestActor: func(*http.Request) (Actor, bool) { return staffActor, true }}))
			router.Post("/api/groups", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) })
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/groups", nil))
			require.Equal(t, http.StatusCreated, response.Code)
			assert.Len(t, target.captured(), state.want, "core action")
		})
	}
}

func TestCollectionPolicyParentsRequireEverySchool(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		schools       []int64
		off           int64
		readErr       bool
		membershipErr bool
		want          bool
	}{
		{name: "both on", schools: []int64{42, 43}, want: true},
		{name: "second off", schools: []int64{42, 43}, off: 43},
		{name: "read error", schools: []int64{42, 43}, readErr: true},
		{name: "membership error", schools: []int64{42}, membershipErr: true},
		{name: "no memberships"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy := CollectionPolicy{
				SchoolEnabled: func(_ context.Context, id int64) (bool, error) {
					if tc.readErr {
						return true, errors.New("unavailable")
					}
					return id != tc.off, nil
				},
				ParentSchools: func(_ context.Context, id int64) (map[int64][]int64, error) {
					if tc.membershipErr {
						return nil, errors.New("unavailable")
					}
					return map[int64][]int64{id: tc.schools}, nil
				},
			}
			actor := Actor{Surface: SurfaceParents, AccountID: 91}
			assert.Equal(t, tc.want, policy.AllowsActor(context.Background(), actor))
			recorder := httptest.NewRecorder()
			PolicyHandler(policy, func(*http.Request) (Actor, bool) { return actor, true })(recorder, httptest.NewRequest(http.MethodGet, "/analytics-policy", nil))
			require.Equal(t, http.StatusOK, recorder.Code)
			assert.Contains(t, recorder.Body.String(), map[bool]string{true: `"enabled":true`, false: `"enabled":false`}[tc.want])
		})
	}
}

func TestSchoolSwitchDoesNotAffectAnotherTenantOrDemo(t *testing.T) {
	t.Parallel()
	policy := CollectionPolicy{SchoolEnabled: func(_ context.Context, id int64) (bool, error) { return id == 43, nil }}
	recorder := &recordingTracker{}
	tracker := GateSchoolTracker(recorder, policy.Demo, policy.SchoolEnabled)
	tracker.Capture("school:42", "room_transfer", map[string]any{"school_id": int64(42)})
	tracker.Capture("school:43", "room_transfer", map[string]any{"school_id": int64(43)})
	require.Len(t, recorder.captured(), 1)
	assert.Equal(t, "school:43", recorder.captured()[0].distinctID)
	policy.Demo = true
	assert.True(t, policy.AllowsActor(context.Background(), Actor{}))
	assert.True(t, policy.AllowsSchool(context.Background(), 42))
	assert.False(t, CollectionPolicy{}.AllowsSchool(context.Background(), 42))
}

func TestPublicEnrollmentUsesTargetSchoolEvenWithAnotherSession(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		enabled   bool
		lookupErr bool
		want      int
	}{
		{"on", true, false, 1}, {"off", false, false, 0}, {"lookup error", true, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := &recordingTracker{}
			policy := CollectionPolicy{
				SchoolEnabled: func(_ context.Context, id int64) (bool, error) {
					require.Equal(t, int64(42), id)
					return tc.enabled, nil
				},
				PublicSchool: func(_ context.Context, slug string) (int64, error) {
					require.Equal(t, "school-a", slug)
					if tc.lookupErr {
						return 0, errors.New("unavailable")
					}
					return 42, nil
				},
			}
			router := chi.NewRouter()
			router.Use(CoreActionMiddleware(CoreActionConfig{Tracker: target, Policy: policy, RequestActor: func(*http.Request) (Actor, bool) { return Actor{Surface: SurfaceOGS, SchoolID: 43}, true }}))
			router.Post("/api/enrollment/{tenantSlug}/submit", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) })
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/enrollment/school-a/submit", nil))
			require.Equal(t, http.StatusCreated, response.Code)
			assert.Len(t, target.captured(), tc.want)
			if tc.want > 0 {
				assert.Equal(t, "42", target.captured()[0].props["school_id"])
			}
		})
	}
}

func TestSchoolTrackerRejectsMissingSchoolMetadata(t *testing.T) {
	t.Parallel()
	target := &recordingTracker{}
	tracker := GateSchoolTracker(target, false, func(context.Context, int64) (bool, error) { return true, nil })
	tracker.Capture("school:42", "room_transfer", nil)
	tracker.Capture("", "$groupidentify", map[string]any{"$group_key": "not-a-school"})
	assert.Empty(t, target.captured())
}
