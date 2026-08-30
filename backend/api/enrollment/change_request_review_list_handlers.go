package enrollment

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
)

// Anmeldungsänderungen in the request module (#2435). The Eltern tab shows all
// parent requests in one list, but this queue sits behind its own permission
// (config:manage instead of users:update), so it keeps its own endpoint rather
// than joining the four-type aggregate — one endpoint per permission boundary.
// The wire shape matches that aggregate exactly (items of {request_type,
// occurred_at, data} plus an opaque next_cursor), so the frontend merges both
// sources into one ordered list and simply leaves this one out when the
// permission is missing.
//
// Deciding still happens in the existing detail view; the list links there.

const (
	// requestTypeEnrollment is the wire name of this request type inside the
	// merged Eltern list.
	requestTypeEnrollment = "enrollment"

	reviewListDefaultLimit = 25
	reviewListMaxLimit     = 100
)

var errInvalidReviewListQuery = errors.New("invalid change request list query")

// openReviewStatuses are the rows the open list displays. A request awaiting a
// parent response remains visible, but cannot be decided by staff yet.
var openReviewStatuses = []string{
	enrollmentModels.ChangeRequestStatusPendingReview,
	enrollmentModels.ChangeRequestStatusNeedsParentResponse,
}

var actionableReviewStatuses = []string{
	enrollmentModels.ChangeRequestStatusPendingReview,
}

// historyReviewStatuses are the terminal ones.
var historyReviewStatuses = []string{
	enrollmentModels.ChangeRequestStatusApproved,
	enrollmentModels.ChangeRequestStatusRejected,
	enrollmentModels.ChangeRequestStatusCancelled,
}

// ChangeRequestReviewEntry is one Anmeldungsänderung in the shared display
// format: who filed what for which child, and who decided it when.
type ChangeRequestReviewEntry struct {
	ID           string                          `json:"id"`
	RequestID    string                          `json:"request_id"`
	Origin       string                          `json:"origin"`
	Status       string                          `json:"status"`
	ChildNames   []string                        `json:"child_names"`
	ChildIDs     []string                        `json:"child_ids,omitempty"`
	Children     []ChangeRequestReviewChildEntry `json:"children"`
	GuardianName string                          `json:"guardian_name,omitempty"`
	ParentNote   *string                         `json:"parent_note,omitempty"`
	DecisionNote *string                         `json:"decision_note,omitempty"`
	// BaseSnapshot, ProposedSnapshot and Diff are the same three fields the
	// detail view compares, so the list can show the real before → after
	// instead of only naming the changed areas: the enrollment as filed and as
	// proposed, plus the changed-key list. The client derives the field rows.
	BaseSnapshot     map[string]any `json:"base_snapshot"`
	ProposedSnapshot map[string]any `json:"proposed_snapshot"`
	Diff             map[string]any `json:"diff"`
	CreatedAt        time.Time      `json:"created_at"`
	// DecidedAt and DecidedByName are set once the request is decided.
	DecidedAt     *time.Time `json:"decided_at,omitempty"`
	DecidedByName string     `json:"decided_by_name,omitempty"`
}

type ChangeRequestReviewChildEntry struct {
	CaseID    string  `json:"case_id"`
	StudentID *string `json:"student_id,omitempty"`
	Name      string  `json:"name"`
}

// ChangeRequestReviewItem wraps one entry in the merged list's envelope.
type ChangeRequestReviewItem struct {
	RequestType string `json:"request_type"`
	// OccurredAt is the instant this row is ordered by: the submission on the
	// open list, the decision in the history. It is what the frontend merges
	// the sources on.
	OccurredAt time.Time                `json:"occurred_at"`
	Data       ChangeRequestReviewEntry `json:"data"`
}

// ChangeRequestReviewPage is the cursor envelope of the review list.
type ChangeRequestReviewPage struct {
	Items []ChangeRequestReviewItem `json:"items"`
	// NextCursor is absent on the last page and only valid for the exact
	// filter set it was produced with.
	NextCursor string `json:"next_cursor,omitempty"`
}

// reviewListCursor is the opaque keyset position, the same wire shape the
// other request histories use.
type reviewListCursor struct {
	Instant time.Time `json:"u"`
	ID      int64     `json:"i"`
}

func parseReviewListQuery(r *http.Request) (enrollmentService.ChangeRequestReviewQuery, error) {
	values := r.URL.Query()
	q := enrollmentService.ChangeRequestReviewQuery{}
	q.Limit = reviewListDefaultLimit

	switch values.Get("view") {
	case "", "open":
		q.Statuses = openReviewStatuses
	case "history":
		q.History = true
	default:
		return q, errInvalidReviewListQuery
	}

	q.Search = strings.TrimSpace(values.Get("search"))

	if err := parseReviewListHistoryFilters(&q, values); err != nil {
		return q, err
	}
	if raw := values.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return q, errInvalidReviewListQuery
		}
		q.Limit = min(n, reviewListMaxLimit)
	}
	if raw := values.Get("cursor"); raw != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			return q, errInvalidReviewListQuery
		}
		var cursor reviewListCursor
		if err := json.Unmarshal(decoded, &cursor); err != nil || cursor.Instant.IsZero() || cursor.ID <= 0 {
			return q, errInvalidReviewListQuery
		}
		q.BeforeInstant = cursor.Instant
		q.BeforeID = cursor.ID
	}
	return q, nil
}

// parseReviewListHistoryFilters reads status/from/to. They only exist in the
// history; passing them on the open list is a client bug and refused loudly.
func parseReviewListHistoryFilters(q *enrollmentService.ChangeRequestReviewQuery, values url.Values) error {
	rawStatus := values.Get("status")
	rawFrom := values.Get("from")
	rawTo := values.Get("to")
	if !q.History {
		if rawStatus != "" || rawFrom != "" || rawTo != "" {
			return errInvalidReviewListQuery
		}
		return nil
	}
	statuses, err := parseReviewListStatuses(rawStatus)
	if err != nil {
		return err
	}
	q.Statuses = statuses
	if rawFrom != "" {
		from, parseErr := timezone.ParseDate(rawFrom)
		if parseErr != nil {
			return errInvalidReviewListQuery
		}
		q.From = from.BerlinMidnight()
	}
	if rawTo != "" {
		to, parseErr := timezone.ParseDate(rawTo)
		if parseErr != nil {
			return errInvalidReviewListQuery
		}
		q.To = to.EndOfDay()
	}
	if !q.From.IsZero() && !q.To.IsZero() && q.From.After(q.To) {
		return errInvalidReviewListQuery
	}
	return nil
}

// parseReviewListStatuses maps the shared status filter onto this queue's own
// statuses: the merged list speaks approved/rejected/withdrawn, this queue
// stores approved/rejected/cancelled.
func parseReviewListStatuses(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return historyReviewStatuses, nil
	}
	statuses := make([]string, 0, len(historyReviewStatuses))
	for _, part := range strings.Split(raw, ",") {
		switch strings.TrimSpace(part) {
		case "approved":
			statuses = append(statuses, enrollmentModels.ChangeRequestStatusApproved)
		case "rejected":
			statuses = append(statuses, enrollmentModels.ChangeRequestStatusRejected)
		case "withdrawn":
			// A cancelled change request is one the family took back — the
			// shared filter's "zurückgezogen".
			statuses = append(statuses, enrollmentModels.ChangeRequestStatusCancelled)
		default:
			return nil, errInvalidReviewListQuery
		}
	}
	return statuses, nil
}

// pendingChangeRequestReviewCount serves
// GET /admin/change-requests/pending-count: how many Anmeldungsänderungen still
// wait for a decision. It backs the badge on the Anfragen sidebar entry, which
// sums it with the other queues the caller may open. Counted in the database,
// so the number stays true past any page size.
func (rs *Resource) pendingChangeRequestReviewCount(w http.ResponseWriter, r *http.Request) {
	if rs.ChangeRequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("change request service not configured")))
		return
	}
	var pending int
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		count, countErr := rs.ChangeRequestService.CountOpenForReview(ctx, actionableReviewStatuses)
		pending = count
		return countErr
	})
	if err != nil {
		mapChangeRequestError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]int{
		"pending_count": pending,
	}, "Pending enrollment change request count retrieved")
}

// listChangeRequestReviewEntries serves GET /admin/change-requests/list.
func (rs *Resource) listChangeRequestReviewEntries(w http.ResponseWriter, r *http.Request) {
	if rs.ChangeRequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("change request service not configured")))
		return
	}
	q, err := parseReviewListQuery(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	var items []*enrollmentService.ChangeRequestReviewItem
	var next *usersService.HistoryCursor
	txErr := rs.runInTenantTx(r, func(ctx context.Context) error {
		rows, cursor, listErr := rs.ChangeRequestService.ListForReview(ctx, q)
		if listErr != nil {
			return listErr
		}
		items, next = rows, cursor
		return nil
	})
	if txErr != nil {
		mapChangeRequestError(w, r, txErr)
		return
	}

	page := ChangeRequestReviewPage{Items: make([]ChangeRequestReviewItem, 0, len(items))}
	for _, item := range items {
		page.Items = append(page.Items, toChangeRequestReviewItem(item, q.History))
	}
	if next != nil {
		page.NextCursor = encodeReviewListCursor(next)
	}
	common.Respond(w, r, http.StatusOK, page, "Enrollment change requests retrieved")
}

func encodeReviewListCursor(cursor *usersService.HistoryCursor) string {
	raw, err := json.Marshal(reviewListCursor{Instant: cursor.UpdatedAt, ID: cursor.ID})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

// orEmptyMap keeps a nil jsonb out of the wire: the client walks these maps,
// and "absent" and "empty" mean the same thing to it.
func orEmptyMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func toChangeRequestReviewItem(item *enrollmentService.ChangeRequestReviewItem, history bool) ChangeRequestReviewItem {
	row := item.ChangeRequest
	entry := ChangeRequestReviewEntry{
		ID:               strconv.FormatInt(row.ID, 10),
		RequestID:        strconv.FormatInt(row.RequestID, 10),
		Origin:           row.Origin,
		Status:           row.Status,
		ChildNames:       item.ChildNames,
		ChildIDs:         formatReviewChildIDs(item.ChildIDs),
		Children:         formatReviewChildren(row.RequestID, item.Children),
		GuardianName:     item.GuardianName,
		ParentNote:       row.ParentNote,
		DecisionNote:     row.AdminDecisionNote,
		BaseSnapshot:     orEmptyMap(row.BaseSnapshot),
		ProposedSnapshot: orEmptyMap(row.ProposedSnapshot),
		Diff:             orEmptyMap(row.Diff),
		CreatedAt:        row.CreatedAt,
	}
	if entry.ChildNames == nil {
		entry.ChildNames = []string{}
	}

	occurredAt := row.CreatedAt
	if history {
		decidedAt := row.DecisionInstant()
		entry.DecidedAt = &decidedAt
		entry.DecidedByName = item.ReviewerName
		occurredAt = decidedAt
	}
	return ChangeRequestReviewItem{
		RequestType: requestTypeEnrollment,
		OccurredAt:  occurredAt,
		Data:        entry,
	}
}

func formatReviewChildren(requestID int64, children []enrollmentService.ChangeRequestReviewChild) []ChangeRequestReviewChildEntry {
	formatted := make([]ChangeRequestReviewChildEntry, 0, len(children))
	for _, child := range children {
		var studentID *string
		if child.StudentID != nil {
			value := strconv.FormatInt(*child.StudentID, 10)
			studentID = &value
		}
		formatted = append(formatted, ChangeRequestReviewChildEntry{
			CaseID:    fmt.Sprintf("%d:%d", requestID, child.RequestChildID),
			StudentID: studentID,
			Name:      child.Name,
		})
	}
	return formatted
}

func formatReviewChildIDs(ids []int64) []string {
	formatted := make([]string, 0, len(ids))
	for _, id := range ids {
		formatted = append(formatted, strconv.FormatInt(id, 10))
	}
	return formatted
}
