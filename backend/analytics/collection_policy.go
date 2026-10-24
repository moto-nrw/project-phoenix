package analytics

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// CollectionPolicy is the school-level privacy gate. Missing or failed reads
// deny collection. Account IDs are used only to find a parent's schools.
type CollectionPolicy struct {
	Demo          bool
	SchoolEnabled func(context.Context, int64) (bool, error)
	ParentSchools func(context.Context, int64) (map[int64][]int64, error)
	PublicSchool  func(context.Context, string) (int64, error)
}

func (p CollectionPolicy) AllowsSchool(ctx context.Context, id int64) bool {
	if p.Demo {
		return true
	}
	if id <= 0 || p.SchoolEnabled == nil {
		return false
	}
	enabled, err := p.SchoolEnabled(ctx, id)
	return err == nil && enabled
}

func (p CollectionPolicy) AllowsActor(ctx context.Context, actor Actor) bool {
	if p.Demo {
		return true
	}
	if actor.Surface != SurfaceParents {
		return p.AllowsSchool(ctx, actor.SchoolID)
	}
	if actor.SchoolID > 0 && !p.AllowsSchool(ctx, actor.SchoolID) {
		return false
	}
	if actor.AccountID <= 0 || p.ParentSchools == nil {
		return false
	}
	memberships, err := p.ParentSchools(ctx, actor.AccountID)
	if err != nil || len(memberships[actor.AccountID]) == 0 {
		return false
	}
	for _, school := range memberships[actor.AccountID] {
		if !p.AllowsSchool(ctx, school) {
			return false
		}
	}
	return true
}

// EnrollmentActor resolves the route target, never a caller-supplied school
// ID. Parents must additionally allow that school, even before membership.
func (p CollectionPolicy) EnrollmentActor(r *http.Request, actor Actor) (Actor, bool) {
	parentEnrollment := actor.Surface == SurfaceParents && chi.URLParam(r, "tenantSlug") != ""
	if p.Demo || (!parentEnrollment && (actor.SchoolID > 0 || actor.Surface != SurfacePublic)) {
		return actor, true
	}
	if p.PublicSchool == nil {
		return Actor{}, false
	}
	id, err := p.PublicSchool(r.Context(), chi.URLParam(r, "tenantSlug"))
	actor.SchoolID = id
	return actor, err == nil && id > 0
}

// PolicyHandler serves only a boolean, using the verified session. Anonymous
// parents/school logins cannot yet determine their schools and stay off.
func PolicyHandler(policy CollectionPolicy, actor func(*http.Request) (Actor, bool)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		current, ok := actor(r)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]bool{"enabled": ok && policy.AllowsActor(r.Context(), current)}})
	}
}

// GateSchoolTracker covers every school-scoped capture, including kiosk
// presence and the worker's groupidentify. The process still owns Close.
func GateSchoolTracker(next interface {
	Capture(string, string, map[string]any)
}, demo bool, enabled func(context.Context, int64) (bool, error)) Tracker {
	return &SchoolTracker{next: next, demo: demo, enabled: enabled}
}

type collectionPermissionKey struct{}

type SchoolTracker struct {
	next interface {
		Capture(string, string, map[string]any)
	}
	demo    bool
	enabled func(context.Context, int64) (bool, error)
}

func (t *SchoolTracker) Capture(id, event string, props map[string]any) {
	t.CaptureContext(context.Background(), id, event, props)
}
func (t *SchoolTracker) CaptureContext(ctx context.Context, id, event string, props map[string]any) {
	school := schoolOfEvent(event, props)
	// Unscoped events are allowed only after the actor policy explicitly
	// admitted them (parents). Missing or malformed school metadata fails closed.
	allowed, _ := ctx.Value(collectionPermissionKey{}).(bool)
	if !t.demo && school <= 0 && !allowed {
		return
	}
	if school > 0 && !t.allowsSchool(ctx, school) {
		return
	}

	if next, ok := t.next.(interface {
		CaptureContext(context.Context, string, string, map[string]any)
	}); ok {
		next.CaptureContext(ctx, id, event, props)
	} else {
		t.next.Capture(id, event, props)
	}
}
func (*SchoolTracker) Close() error { return nil }
func schoolOfEvent(event string, props map[string]any) int64 {
	value := props["school_id"]
	if event == groupIdentifyEvent {
		value = props["$group_key"]
	}
	switch id := value.(type) {
	case int64:
		return id
	case string:
		parsed, _ := strconv.ParseInt(id, 10, 64)
		return parsed
	default:
		return 0
	}
}

func (t *SchoolTracker) allowsSchool(ctx context.Context, school int64) bool {
	if t.demo {
		return true
	}
	if school <= 0 || t.enabled == nil {
		return false
	}
	enabled, err := t.enabled(ctx, school)
	return err == nil && enabled
}
