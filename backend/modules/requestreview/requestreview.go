// Package requestreview is the shared request-review read projection (#2705):
// the one staff-facing list of every parent request awaiting or carrying a
// decision — Stammdaten, Betreuungszeiten, Angebote, Abwesenheiten and the
// office's own booking corrections — showing origin, type, child, requested
// change and decision. The owner queues stay separate; the projection owns
// the merged order, the keyset paging, the permission narrowing, the bulk and
// past consequences, the conflict grouping and the badge count. Every foreign
// fact enters through a consumer-owned port; the projection persists nothing
// and never writes.
package requestreview

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// ErrNotConfigured reports a missing queue or access port.
var ErrNotConfigured = errors.New("request review projection is not configured")

// Query is the single public capability of the projection.
type Query interface {
	// ListRequests serves the open queue (view=open) or the decision history
	// (view=history) as one keyset page.
	ListRequests(ctx context.Context, query ListQuery) (Page, error)
	// PendingCount is the number of open requests across the queues the
	// caller may review — the sidebar badge.
	PendingCount(ctx context.Context) (int, error)
}

// Dependencies are the consumer-owned ports the projection reads through.
// Queues and Access are required. Students and FamilyProtection are
// optional: without them the open page omits group names and reports no
// Familienschutz. Today defaults to the Berlin calendar day of the wall
// clock; it is resolved once per call so the queues' urgency phase and the
// rows' urgency and past flags never straddle midnight.
type Dependencies struct {
	Queues           Queues
	Access           Access
	Students         StudentDirectory
	FamilyProtection FamilyProtection
	Today            func() Date
}

type service struct{ deps Dependencies }

// New builds the projection. Missing wiring surfaces as ErrNotConfigured on
// the first call, never as a partial list.
func New(deps Dependencies) Query {
	if deps.Today == nil {
		deps.Today = func() Date { return Date(timezone.TodayDate()) }
	}
	return &service{deps: deps}
}

// Wire names of the request types. They double as the cursor's map keys.
const (
	TypeMasterData   = "master_data"
	TypeCareSchedule = "care_schedule"
	TypeOffering     = "offering"
	TypeExcused      = "excused"
	// TypeDirectCorrection is not a request: it is the office's own
	// correction to a child's bookings (#2436). History only.
	TypeDirectCorrection = "direct_correction"
)

// TypeOrder is the canonical type order — it decides the deterministic
// tie-break when two rows of different types share an instant.
var TypeOrder = []string{
	TypeMasterData,
	TypeCareSchedule,
	TypeOffering,
	TypeExcused,
	TypeDirectCorrection,
}

// Stable codes and German sentences of the bulk consequences the projection
// itself decides. Other reasons come from the owner queues.
const (
	BulkIneligiblePast     = "past"
	bulkIneligiblePastText = "Diese Anfrage betrifft nur vergangene Tage."
)

// Item is one request of any of the four types. Data holds the unchanged
// per-type projection (queue shape for view=open, history shape for
// view=history), discriminated by RequestType.
type Item struct {
	RequestType string `json:"request_type"`
	// OccurredAt is the instant the row is ordered by: the submission on the
	// open view, the decision in the history. The client merges this list with
	// the separately gated Anmeldungsänderungen (#2435) on it.
	OccurredAt      time.Time `json:"occurred_at"`
	StudentID       string    `json:"student_id"`
	StudentName     string    `json:"student_name"`
	GroupName       string    `json:"group_name,omitempty"`
	ExpectedVersion string    `json:"expected_version"`
	UrgentToday     bool      `json:"urgent_today"`
	// Past marks a request whose whole scope lies before today: approving it
	// would change nothing, so the client offers reject or "als erledigt".
	Past         bool `json:"past"`
	BulkEligible bool `json:"bulk_eligible"`
	// BulkIneligibleReason is a STABLE CODE (past, conflict, stale,
	// single_only, child_unavailable, access_revoked). The German sentence
	// travels separately so a client can render an unknown code.
	BulkIneligibleReason string `json:"bulk_ineligible_reason,omitempty"`
	BulkIneligibleText   string `json:"bulk_ineligible_text,omitempty"`
	// ConflictKeys name what this request would write. Two open requests of
	// one child sharing a key contradict each other and must be resolved
	// together, never one after the other.
	ConflictKeys []string `json:"conflict_keys,omitempty"`
	// ConflictKey is the key this request is most contended on, and
	// ConflictGroupSize how many open requests of this child share it
	// (1 = nothing to resolve). The client groups on ConflictKey.
	ConflictKey       string `json:"conflict_key,omitempty"`
	ConflictGroupSize int    `json:"conflict_group_size,omitempty"`
	// CurrentValueChanged warns that the OGS changed this value after the
	// request was filed. Emitted for Stammdaten and Abwesenheiten, where a
	// submission baseline exists; omitted for the other types.
	CurrentValueChanged *bool `json:"current_value_changed,omitempty"`
	// CurrentStatusByDate is what each requested day looks like today
	// (present, sick, excused, class_trip). Absence requests only.
	CurrentStatusByDate map[string]string `json:"current_status_by_date,omitempty"`
	// CanCorrect marks a decided request whose decision staff can still
	// rewrite. History rows only; false where the type keeps no pre-decision
	// state to revert to, and for rows that were never really decided
	// (withdrawn, marked done, closed by a care end).
	CanCorrect      bool `json:"can_correct,omitempty"`
	FamilyProtected bool `json:"family_protected"`
	Data            any  `json:"data"`
	// studentID is StudentID as the owner queues report it. The wire carries
	// the string (backend int64 IDs map to frontend strings); the decorations
	// key on this instead of parsing the string back.
	studentID int64
}

// Page is the cursor envelope of the list.
type Page struct {
	Items []Item `json:"items"`
	// NextCursor is absent on the last page. It is only valid for the exact
	// filter set it was produced with.
	NextCursor string `json:"next_cursor,omitempty"`
	// ReviewAccess tells the client WHY an open list may be empty:
	// "admin" (school-wide), "group_leader" (own groups only), or "none"
	// (the school has not enabled group-leader decisions). History omits it.
	ReviewAccess string `json:"review_access,omitempty"`
}

func (s *service) configured() error {
	q := s.deps.Queues
	if q.MasterData == nil || q.CareSchedule == nil || q.Offering == nil || q.Excused == nil ||
		q.DirectCorrections == nil || s.deps.Access == nil {
		return ErrNotConfigured
	}
	return nil
}

// ListRequests serves the unified Eltern request list. Which queues
// contribute follows each queue's own gate: a caller without the write-queue
// right gets only the excused queue (#2232). Per-child scoping happens inside
// the owner queues, exactly as on the per-type routes.
func (s *service) ListRequests(ctx context.Context, query ListQuery) (Page, error) {
	if err := s.configured(); err != nil {
		return Page{}, err
	}
	if err := query.validate(); err != nil {
		return Page{}, err
	}
	caller, err := s.deps.Access.Caller(ctx)
	if err != nil {
		return Page{}, err
	}
	if !caller.ReviewsWriteQueues {
		query.Types = intersectTypes(query.Types, TypeExcused)
	}
	query.today = s.deps.Today()

	page, err := s.page(ctx, &query)
	if err != nil {
		return Page{}, err
	}
	if !query.History {
		if err := s.decorateOpenPage(ctx, &page, query.Types); err != nil {
			return Page{}, err
		}
	}
	return page, nil
}

// PendingCount sums every queue the caller can actually open on the
// Änderungsanfragen page: a request missing from the count is a request
// nobody looks at until someone happens to open the page. The excused queue
// always counts (its route also accepts users:absence); the three
// users:update-gated queues count only for callers who hold that right,
// because a badge for a queue that answers 403 is worse than no badge.
func (s *service) PendingCount(ctx context.Context) (int, error) {
	if err := s.configured(); err != nil {
		return 0, err
	}
	pending, err := s.deps.Queues.Excused.OpenCount(ctx)
	if err != nil {
		return 0, err
	}
	caller, err := s.deps.Access.Caller(ctx)
	if err != nil {
		return 0, err
	}
	if !caller.ReviewsWriteQueues {
		return pending, nil
	}
	for _, queue := range []Queue{s.deps.Queues.MasterData, s.deps.Queues.CareSchedule, s.deps.Queues.Offering} {
		count, err := queue.OpenCount(ctx)
		if err != nil {
			return 0, err
		}
		pending += count
	}
	return pending, nil
}
