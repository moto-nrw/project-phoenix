package requestreview

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// ErrInvalidQuery reports a malformed listing query: an unknown view, type
// or status, a bad date, a history-only filter on the open view, a
// non-positive limit or student ID, or a cursor this projection did not mint.
var ErrInvalidQuery = errors.New("invalid change request list query")

// The list defaults to a screenful and caps the page size against abusive
// limits; the client loads more via the cursor.
const (
	DefaultLimit = 25
	MaxLimit     = 100
)

// ListQuery is the parsed, validated query of the list. Build it with
// ParseListQuery; a zero value lists the open queue with every type.
type ListQuery struct {
	// History selects the decided requests instead of the open queue.
	History bool
	// Search matches the child's name.
	Search string
	// StudentID limits the list to one child — the Kinderkartei's
	// Änderungsprotokoll (#2437). Zero = every child the caller may see.
	StudentID int64
	// Types are the request types to serve, in canonical order. Empty means
	// every type (the open view never serves corrections).
	Types []string
	// Statuses filters the history by canonical status (approved, rejected,
	// withdrawn); empty means all. Auto-applied rows count as approved.
	Statuses map[string]struct{}
	// From and To bound the history's decided-at instant; zero = unbounded.
	From, To time.Time
	// Limit is the page size; zero means DefaultLimit.
	Limit int
	// Cursor is the opaque next_cursor of the previous page, "" for the
	// first page.
	Cursor string

	cursor     pageCursor
	urgentOnly *bool
	// typesNormalized marks Types as already defaulted, ordered and stripped
	// of corrections, so a second validate keeps an explicitly emptied set
	// empty instead of widening it to every type.
	typesNormalized bool
	// today is the calendar day the urgency phase is judged against,
	// resolved once per call by the projection.
	today Date
}

// ParseListQuery reads the wire query of the list route.
func ParseListQuery(values url.Values) (ListQuery, error) {
	q := ListQuery{Limit: DefaultLimit}

	switch values.Get("view") {
	case "", "open":
	case "history":
		q.History = true
	default:
		return q, ErrInvalidQuery
	}

	q.Search = strings.TrimSpace(values.Get("search"))

	if raw := values.Get("student_id"); raw != "" {
		studentID, convErr := strconv.ParseInt(raw, 10, 64)
		if convErr != nil || studentID <= 0 {
			return q, ErrInvalidQuery
		}
		q.StudentID = studentID
	}

	types, err := parseTypes(values.Get("types"))
	if err != nil {
		return q, err
	}
	q.Types = types

	if err := parseHistoryFilters(&q, values); err != nil {
		return q, err
	}

	if raw := values.Get("limit"); raw != "" {
		n, convErr := strconv.Atoi(raw)
		if convErr != nil || n <= 0 {
			return q, ErrInvalidQuery
		}
		q.Limit = n
	}

	q.Cursor = values.Get("cursor")
	if err := q.validate(); err != nil {
		return q, err
	}
	return q, nil
}

// validate normalizes a query built by hand or by ParseListQuery: it fills
// the defaults, orders the types, drops corrections from the open view and
// decodes the cursor.
func (q *ListQuery) validate() error {
	if q.Limit <= 0 {
		q.Limit = DefaultLimit
	}
	q.Limit = min(q.Limit, MaxLimit)
	if !q.typesNormalized {
		if len(q.Types) == 0 {
			q.Types = append([]string(nil), TypeOrder...)
		} else {
			for _, typ := range q.Types {
				if !isRequestType(typ) {
					return ErrInvalidQuery
				}
			}
			q.Types = intersectTypes(TypeOrder, q.Types...)
		}
		// Corrections have no open state — the working list must never show
		// them, not even when a client asks for the type explicitly.
		if !q.History {
			q.Types = removeType(q.Types, TypeDirectCorrection)
		}
		q.typesNormalized = true
	}
	if !q.History {
		if len(q.Statuses) > 0 || !q.From.IsZero() || !q.To.IsZero() {
			return ErrInvalidQuery
		}
	}
	if !q.From.IsZero() && !q.To.IsZero() && q.From.After(q.To) {
		return ErrInvalidQuery
	}
	q.cursor = nil
	if q.Cursor != "" {
		cursor, err := decodeCursor(q.Cursor)
		if err != nil {
			return err
		}
		q.cursor = cursor
	}
	return nil
}

func parseTypes(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	requested := make([]string, 0, 4)
	for part := range strings.SplitSeq(raw, ",") {
		typ := strings.TrimSpace(part)
		if !isRequestType(typ) {
			return nil, ErrInvalidQuery
		}
		requested = append(requested, typ)
	}
	return requested, nil
}

// parseHistoryFilters reads status/from/to — history-only filters; their
// presence on the open view is a client bug and rejected loudly.
func parseHistoryFilters(q *ListQuery, values url.Values) error {
	rawStatus := values.Get("status")
	rawFrom := values.Get("from")
	rawTo := values.Get("to")
	if !q.History && (rawStatus != "" || rawFrom != "" || rawTo != "") {
		return ErrInvalidQuery
	}
	if rawStatus != "" {
		statuses := make(map[string]struct{})
		for part := range strings.SplitSeq(rawStatus, ",") {
			status := strings.TrimSpace(part)
			switch status {
			case "approved", "rejected", "withdrawn":
				statuses[status] = struct{}{}
			default:
				return ErrInvalidQuery
			}
		}
		q.Statuses = statuses
	}
	if rawFrom != "" {
		from, err := timezone.ParseDate(rawFrom)
		if err != nil {
			return ErrInvalidQuery
		}
		q.From = from.BerlinMidnight()
	}
	if rawTo != "" {
		to, err := timezone.ParseDate(rawTo)
		if err != nil {
			return ErrInvalidQuery
		}
		q.To = to.EndOfDay()
	}
	return nil
}

// matches applies the row filters that stay in Go because they are per-type
// semantics rather than columns: the status set (auto-applied counts as
// accepted) and the decided-at range, which reads the same reviewed_at ??
// updated_at fallback the response shows. Determinism matters: the cursor may
// advance past filtered rows, which is only safe when the same row is filtered
// identically on the next request.
func (q *ListQuery) matches(row *Row) bool {
	if len(q.Statuses) > 0 {
		status := row.Status
		// Auto-applied rows are shown (and filtered) as accepted — the change
		// went through, just without a manual reviewer.
		if status == "auto_applied" {
			status = "approved"
		}
		if _, ok := q.Statuses[status]; !ok {
			return false
		}
	}
	if !q.From.IsZero() && row.DecidedAt.Before(q.From) {
		return false
	}
	if !q.To.IsZero() && row.DecidedAt.After(q.To) {
		return false
	}
	return true
}

// queueFilter is the part of the query the owner queues evaluate themselves:
// which children, plus the urgency phase; the source fills in the page size
// and the keyset.
func (q *ListQuery) queueFilter() QueueFilter {
	return QueueFilter{
		StudentID: q.StudentID, Search: q.Search,
		UrgentOnly: q.urgentOnly, UrgentDate: q.today.String(),
	}
}

func isRequestType(typ string) bool {
	return slices.Contains(TypeOrder, typ)
}

func typeRank(typ string) int {
	for i, known := range TypeOrder {
		if typ == known {
			return i
		}
	}
	return len(TypeOrder)
}

func removeType(types []string, drop string) []string {
	kept := make([]string, 0, len(types))
	for _, typ := range types {
		if typ != drop {
			kept = append(kept, typ)
		}
	}
	return kept
}

func intersectTypes(types []string, allowed ...string) []string {
	kept := make([]string, 0, len(types))
	for _, typ := range types {
		if slices.Contains(allowed, typ) {
			kept = append(kept, typ)
		}
	}
	return kept
}

// Cursor encoding. The whole cursor is base64url-encoded JSON, opaque so
// clients cannot build one by hand and the encoding may change.

// cursorPosition is one type's keyset position inside the page cursor. On
// the open view the instant is the row's created_at; the payload field name
// (u) just follows the shared wire shape.
type cursorPosition struct {
	UpdatedAt time.Time `json:"u"`
	ID        int64     `json:"i"`
}

// pageCursor maps request types to their keyset position — the last DB row
// of that type the client has consumed. A present nil position means that
// the source had buffered rows but none had been consumed yet, so it resumes
// from the top without repeating rows emitted by another source. Types absent
// from the map start from the top. The normal-phase key marks an open page
// that has left the urgent phase.
type pageCursor map[string]*cursorPosition

const normalPhaseCursorKey = "_normal"

func encodeCursor(cursor pageCursor) string {
	if len(cursor) == 0 {
		return ""
	}
	raw, err := json.Marshal(cursor)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeCursor(raw string) (pageCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, ErrInvalidQuery
	}
	var cursor pageCursor
	if err := json.Unmarshal(decoded, &cursor); err != nil || len(cursor) == 0 {
		return nil, ErrInvalidQuery
	}
	for typ, pos := range cursor {
		if typ == normalPhaseCursorKey && pos == nil {
			continue
		}
		if !isRequestType(typ) || (pos != nil && (pos.UpdatedAt.IsZero() || pos.ID <= 0)) {
			return nil, ErrInvalidQuery
		}
	}
	return cursor, nil
}

func withNormalPhaseCursor(raw string) string {
	if raw == "" {
		return ""
	}
	cursor, err := decodeCursor(raw)
	if err != nil {
		return ""
	}
	cursor[normalPhaseCursorKey] = nil
	return encodeCursor(cursor)
}
