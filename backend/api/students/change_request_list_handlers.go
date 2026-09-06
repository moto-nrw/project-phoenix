package students

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	absenceService "github.com/moto-nrw/project-phoenix/services/absence"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

// Aggregated Eltern request list (#2432): GET /students/change-requests
// serves all four guardian queues (Stammdaten, Betreuungszeiten, Angebote,
// Abwesenheiten) as ONE list — open or history — with server-side child
// name search, request-type filter and, in the history, status and decided-at
// range filters. The tables stay separate; only the read model is unified.
// Each item carries the unchanged per-type projection under "data", so the
// existing decide dialogs keep working against the per-type decide routes.

// Wire names of the four request types. They double as the cursor's map keys.
const (
	requestTypeMasterData   = "master_data"
	requestTypeCareSchedule = "care_schedule"
	requestTypeOffering     = "offering"
	requestTypeExcused      = "excused"
	// requestTypeDirectCorrection is not a request: it is the office's own
	// correction to a child's bookings (#2436). History only.
	requestTypeDirectCorrection = "direct_correction"
)

// aggregatedRequestTypeOrder is the canonical type order — it decides the
// deterministic tie-break when two rows of different types share an instant.
var aggregatedRequestTypeOrder = []string{
	requestTypeMasterData,
	requestTypeCareSchedule,
	requestTypeOffering,
	requestTypeExcused,
	requestTypeDirectCorrection,
}

const aggregatedNormalPhaseCursorKey = "_normal"

const (
	// aggregatedFetchLimit is the per-service page size of one internal fetch;
	// larger than the response limit so a page still fills when the per-child
	// scope drops rows.
	aggregatedFetchLimit = 50
	// aggregatedMaxFetches bounds the DB scanning per type and request. Child
	// name and student filters run in SQL, so what still discards rows here is
	// the status/decided-at filter and the caller's per-child scope. A page may
	// come back underfilled with a next_cursor; the client's "load more"
	// continues from there.
	aggregatedMaxFetches = 4
)

var errInvalidAggregatedQuery = errors.New("invalid change request list query")

// AggregatedChangeRequestItem is one request of any of the four types. Data
// holds the unchanged per-type projection (queue shape for view=open, history
// shape for view=history), discriminated by RequestType.
type AggregatedChangeRequestItem struct {
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
}

// AggregatedChangeRequestPage is the cursor envelope of the aggregated list.
type AggregatedChangeRequestPage struct {
	Items []AggregatedChangeRequestItem `json:"items"`
	// NextCursor is absent on the last page. It is only valid for the exact
	// filter set it was produced with.
	NextCursor string `json:"next_cursor,omitempty"`
	// ReviewAccess tells the client WHY an open list may be empty:
	// "admin" (school-wide), "group_leader" (own groups only), or "none"
	// (the school has not enabled group-leader decisions). History omits it.
	ReviewAccess string `json:"review_access,omitempty"`
}

// aggregatedCursor maps request types to their keyset position — the last DB
// row of that type the client has consumed. A present nil position means that
// the source had buffered rows but none had been consumed yet, so it resumes
// from the top without repeating rows emitted by another source. Types absent
// from the map start from the top. Opaque on the wire (base64url JSON), like
// the per-type history cursor. On the open view the position instant is the
// row's created_at; the payload field name (u) just follows the shared wire
// shape.
type aggregatedCursor map[string]*historyCursorPayload

func encodeAggregatedCursor(cursor aggregatedCursor) string {
	if len(cursor) == 0 {
		return ""
	}
	raw, err := json.Marshal(cursor)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeAggregatedCursor(raw string) (aggregatedCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, errInvalidAggregatedQuery
	}
	var cursor aggregatedCursor
	if err := json.Unmarshal(decoded, &cursor); err != nil || len(cursor) == 0 {
		return nil, errInvalidAggregatedQuery
	}
	for typ, pos := range cursor {
		if typ == aggregatedNormalPhaseCursorKey && pos == nil {
			continue
		}
		if !isAggregatedRequestType(typ) || (pos != nil && (pos.UpdatedAt.IsZero() || pos.ID <= 0)) {
			return nil, errInvalidAggregatedQuery
		}
	}
	return cursor, nil
}

func isAggregatedRequestType(typ string) bool {
	return slices.Contains(aggregatedRequestTypeOrder, typ)
}

// aggregatedListQuery is the parsed, validated query of the aggregate route.
type aggregatedListQuery struct {
	history bool
	search  string
	// studentID limits the list to one child — the Kinderkartei's
	// Änderungsprotokoll (#2437). Zero = every child the caller may see.
	studentID  int64
	types      []string
	statuses   map[string]struct{} // canonical: approved / rejected / withdrawn; empty = all
	from, to   time.Time           // decided-at bounds; zero = unbounded
	limit      int
	cursor     aggregatedCursor
	urgentOnly *bool
}

func parseAggregatedListQuery(r *http.Request) (aggregatedListQuery, error) {
	q := aggregatedListQuery{limit: historyDefaultLimit}
	values := r.URL.Query()

	switch values.Get("view") {
	case "", "open":
	case "history":
		q.history = true
	default:
		return q, errInvalidAggregatedQuery
	}

	q.search = strings.TrimSpace(values.Get("search"))

	if raw := values.Get("student_id"); raw != "" {
		studentID, convErr := strconv.ParseInt(raw, 10, 64)
		if convErr != nil || studentID <= 0 {
			return q, errInvalidAggregatedQuery
		}
		q.studentID = studentID
	}

	types, err := parseAggregatedTypes(values.Get("types"))
	if err != nil {
		return q, err
	}
	q.types = types

	if err := parseAggregatedHistoryFilters(&q, values); err != nil {
		return q, err
	}

	// Corrections have no open state — the working list must never show them,
	// not even when a client asks for the type explicitly.
	if !q.history {
		q.types = removeType(q.types, requestTypeDirectCorrection)
	}

	if raw := values.Get("limit"); raw != "" {
		n, convErr := strconv.Atoi(raw)
		if convErr != nil || n <= 0 {
			return q, errInvalidAggregatedQuery
		}
		q.limit = min(n, historyMaxLimit)
	}

	if raw := values.Get("cursor"); raw != "" {
		cursor, cursorErr := decodeAggregatedCursor(raw)
		if cursorErr != nil {
			return q, cursorErr
		}
		q.cursor = cursor
	}
	return q, nil
}

func parseAggregatedTypes(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return append([]string(nil), aggregatedRequestTypeOrder...), nil
	}
	requested := make(map[string]struct{})
	for part := range strings.SplitSeq(raw, ",") {
		typ := strings.TrimSpace(part)
		if !isAggregatedRequestType(typ) {
			return nil, errInvalidAggregatedQuery
		}
		requested[typ] = struct{}{}
	}
	types := make([]string, 0, len(requested))
	for _, typ := range aggregatedRequestTypeOrder {
		if _, ok := requested[typ]; ok {
			types = append(types, typ)
		}
	}
	return types, nil
}

// parseAggregatedHistoryFilters reads status/from/to — history-only filters;
// their presence on the open view is a client bug and rejected loudly.
func parseAggregatedHistoryFilters(q *aggregatedListQuery, values url.Values) error {
	rawStatus := values.Get("status")
	rawFrom := values.Get("from")
	rawTo := values.Get("to")
	if !q.history && (rawStatus != "" || rawFrom != "" || rawTo != "") {
		return errInvalidAggregatedQuery
	}
	if rawStatus != "" {
		statuses, err := parseAggregatedStatuses(rawStatus)
		if err != nil {
			return err
		}
		q.statuses = statuses
	}
	return parseAggregatedDateRange(q, rawFrom, rawTo)
}

func parseAggregatedStatuses(raw string) (map[string]struct{}, error) {
	statuses := make(map[string]struct{})
	for part := range strings.SplitSeq(raw, ",") {
		status := strings.TrimSpace(part)
		switch status {
		case "approved", "rejected", "withdrawn":
			statuses[status] = struct{}{}
		default:
			return nil, errInvalidAggregatedQuery
		}
	}
	return statuses, nil
}

func parseAggregatedDateRange(q *aggregatedListQuery, rawFrom, rawTo string) error {
	if rawFrom != "" {
		from, err := timezone.ParseDate(rawFrom)
		if err != nil {
			return errInvalidAggregatedQuery
		}
		q.from = from.BerlinMidnight()
	}
	if rawTo != "" {
		to, err := timezone.ParseDate(rawTo)
		if err != nil {
			return errInvalidAggregatedQuery
		}
		q.to = to.EndOfDay()
	}
	if !q.from.IsZero() && !q.to.IsZero() && q.from.After(q.to) {
		return errInvalidAggregatedQuery
	}
	return nil
}

// aggregatedRow is one request of any type, carrying just enough to merge,
// filter and serialize. sortTime is the keyset instant of the underlying DB
// row: created_at on the open view, updated_at on the history view.
type aggregatedRow struct {
	typ                  string
	sortTime             time.Time
	id                   int64
	studentID            int64
	studentName          string
	status               string
	version              time.Time
	urgentToday          bool
	past                 bool
	bulkEligible         bool
	bulkIneligibleReason string
	bulkIneligibleText   string
	conflictKeys         []string
	currentValueChanged  *bool
	currentStatusByDate  map[string]string
	canCorrect           bool
	decidedAt            time.Time // history only
	data                 any
}

// queueFilters is the part of the query the repositories evaluate themselves:
// which children, plus the page size and keyset the source fills in.
func (q *aggregatedListQuery) queueFilters() modelBase.RequestQueueFilters {
	return modelBase.RequestQueueFilters{
		StudentID: q.studentID, Search: q.search,
		UrgentOnly: q.urgentOnly, UrgentDate: timezone.TodayDate().String(),
	}
}

// matches applies the row filters that stay in Go because they are per-type
// semantics rather than columns: the status set (auto-applied counts as
// accepted) and the decided-at range, which reads the same reviewed_at ??
// updated_at fallback the response shows. Determinism matters: the cursor may
// advance past filtered rows, which is only safe when the same row is filtered
// identically on the next request.
func (q *aggregatedListQuery) matches(row *aggregatedRow) bool {
	if len(q.statuses) > 0 {
		status := row.status
		// Auto-applied rows are shown (and filtered) as accepted — the change
		// went through, just without a manual reviewer.
		if status == "auto_applied" {
			status = "approved"
		}
		if _, ok := q.statuses[status]; !ok {
			return false
		}
	}
	if !q.from.IsZero() && row.decidedAt.Before(q.from) {
		return false
	}
	if !q.to.IsZero() && row.decidedAt.After(q.to) {
		return false
	}
	return true
}

func aggregatedTypeRank(typ string) int {
	for i, known := range aggregatedRequestTypeOrder {
		if typ == known {
			return i
		}
	}
	return len(aggregatedRequestTypeOrder)
}

// rowBefore reports whether a is emitted before b (newer first; deterministic
// type-then-id tie-break).
func rowBefore(a, b *aggregatedRow) bool {
	if !a.sortTime.Equal(b.sortTime) {
		return a.sortTime.After(b.sortTime)
	}
	if a.typ != b.typ {
		return aggregatedTypeRank(a.typ) < aggregatedTypeRank(b.typ)
	}
	return a.id > b.id
}

// listAggregatedChangeRequests serves the unified Eltern request list. The
// route is gated users:update OR users:absence like the pending-count badge;
// which queues actually contribute follows each queue's own gate: an
// absence-only caller gets only the excused queue (#2232). Per-child scoping
// happens inside the four services, exactly as on the per-type routes.
func (rs *Resource) listAggregatedChangeRequests(w http.ResponseWriter, r *http.Request) {
	if rs.MasterDataReviewService == nil || rs.CareRequestService == nil ||
		rs.OfferingChangeService == nil || rs.ExcusedRequestService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("change request services not configured")))
		return
	}
	q, err := parseAggregatedListQuery(r)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	ctx := r.Context()
	if !authorize.HasPermission(permissions.UsersUpdate, jwt.PermissionsFromCtx(ctx)) {
		q.types = intersectTypes(q.types, requestTypeExcused)
	}

	page, err := rs.aggregatedPage(ctx, &q)
	if err != nil {
		renderError(w, r, parentRequestQueueErrorRenderer(err))
		return
	}
	if !q.history {
		if err := rs.decorateOpenPage(ctx, &page, q.types); err != nil {
			renderError(w, r, parentRequestQueueErrorRenderer(err))
			return
		}
	}
	common.Respond(w, r, http.StatusOK, page, "Change requests retrieved")
}

func (rs *Resource) decorateStudentGroups(ctx context.Context, page *AggregatedChangeRequestPage) error {
	studentIDs := aggregatedStudentIDs(page.Items)
	students, err := rs.PersonService.GetStudentsByIDs(ctx, studentIDs)
	if err != nil {
		return fmt.Errorf("load students for request queue groups: %w", err)
	}
	groupIDs := make([]int64, 0, len(students))
	for _, student := range students {
		if student != nil && student.GroupID != nil {
			groupIDs = append(groupIDs, *student.GroupID)
		}
	}
	groups, err := rs.EducationService.GetGroupsByIDs(ctx, groupIDs)
	if err != nil {
		return fmt.Errorf("load groups for request queue: %w", err)
	}
	for i := range page.Items {
		id, _ := strconv.ParseInt(page.Items[i].StudentID, 10, 64)
		student := students[id]
		if student != nil && student.GroupID != nil && groups[*student.GroupID] != nil {
			page.Items[i].GroupName = groups[*student.GroupID].Name
		}
	}
	return nil
}

func aggregatedStudentIDs(items []AggregatedChangeRequestItem) []int64 {
	studentIDs := make([]int64, 0, len(items))
	seen := make(map[int64]struct{}, len(items))
	for _, item := range items {
		id, err := strconv.ParseInt(item.StudentID, 10, 64)
		if err == nil && id > 0 {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				studentIDs = append(studentIDs, id)
			}
		}
	}
	return studentIDs
}

func (rs *Resource) decorateFamilyProtection(ctx context.Context, page *AggregatedChangeRequestPage) error {
	studentIDs := aggregatedStudentIDs(page.Items)
	current, err := rs.FamilyProtectionService.Current(ctx, studentIDs)
	if err != nil {
		return fmt.Errorf("load family protection for request queue: %w", err)
	}
	for i := range page.Items {
		id, _ := strconv.ParseInt(page.Items[i].StudentID, 10, 64)
		page.Items[i].FamilyProtected = current[id] != nil && current[id].Enabled
	}
	return nil
}

// newAggregatedItem is the single place a row becomes a wire item, so both
// views agree on what occurred_at means.
func newAggregatedItem(row *aggregatedRow) AggregatedChangeRequestItem {
	return AggregatedChangeRequestItem{
		RequestType:          row.typ,
		OccurredAt:           row.sortTime,
		StudentID:            strconv.FormatInt(row.studentID, 10),
		StudentName:          row.studentName,
		ExpectedVersion:      userService.ParentRequestVersion(row.version),
		UrgentToday:          row.urgentToday,
		Past:                 row.past,
		BulkEligible:         row.bulkEligible,
		BulkIneligibleReason: row.bulkIneligibleReason,
		BulkIneligibleText:   row.bulkIneligibleText,
		ConflictKeys:         row.conflictKeys,
		CurrentValueChanged:  row.currentValueChanged,
		CurrentStatusByDate:  row.currentStatusByDate,
		CanCorrect:           row.canCorrect,
		Data:                 row.data,
	}
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

// mapAggregatedRows adapts one typed service result to the shared row shape.
func mapAggregatedRows[T any](items []T, build func(T) aggregatedRow) []aggregatedRow {
	rows := make([]aggregatedRow, 0, len(items))
	for _, item := range items {
		rows = append(rows, build(item))
	}
	return rows
}

func masterDataPendingRow(item *userService.MasterDataReviewItem) aggregatedRow {
	return aggregatedRow{
		typ:                  requestTypeMasterData,
		sortTime:             item.Request.CreatedAt,
		id:                   item.Request.ID,
		studentID:            item.Request.StudentID,
		studentName:          item.FirstName + " " + item.LastName,
		status:               item.Request.Status,
		version:              item.Request.UpdatedAt,
		bulkEligible:         item.BulkEligible,
		bulkIneligibleReason: item.BulkIneligibleReason,
		bulkIneligibleText:   item.BulkIneligibleText,
		conflictKeys:         masterDataConflictKeys(item),
		currentValueChanged:  item.CurrentValueChanged,
		data:                 toMasterDataChangeRequestResponse(item),
	}
}

func carePendingRow(item *scheduleService.CareRequestReviewItem) aggregatedRow {
	row := aggregatedRow{
		typ:                  requestTypeCareSchedule,
		sortTime:             item.Request.CreatedAt,
		id:                   item.Request.ID,
		studentID:            item.Request.StudentID,
		studentName:          item.FirstName + " " + item.LastName,
		status:               item.Request.Status,
		version:              item.Request.UpdatedAt,
		urgentToday:          careRequestUrgentToday(item),
		bulkIneligibleReason: userService.BulkIneligibleSingleOnly,
		bulkIneligibleText:   "Betreuungszeiten müssen einzeln geprüft werden.",
		conflictKeys:         careConflictKeys(item),
		data:                 toCareRequestResponse(item),
	}
	markPast(&row, careScopeEnd(item))
	return row
}

func offeringPendingRow(item *enrollmentService.OfferingChangeView) aggregatedRow {
	row := aggregatedRow{
		typ:                  requestTypeOffering,
		sortTime:             item.Request.CreatedAt,
		id:                   item.Request.ID,
		studentID:            item.Request.StudentID,
		studentName:          item.StudentName,
		status:               item.Request.Status,
		version:              item.Request.UpdatedAt,
		urgentToday:          !timezone.Date(item.Request.EffectiveFrom).After(timezone.TodayDate()),
		bulkIneligibleReason: userService.BulkIneligibleSingleOnly,
		bulkIneligibleText:   "Angebote müssen einzeln geprüft werden.",
		conflictKeys:         offeringConflictKeys(item),
		data:                 toOfferingRequestResponse(item),
	}
	markPast(&row, timezone.Date(item.Request.EffectiveFrom))
	return row
}

func excusedPendingRow(item *absenceService.ExcusedRequestReviewItem) aggregatedRow {
	today := timezone.TodayDate()
	urgent := false
	for _, date := range item.Request.Dates {
		urgent = urgent || date == today
	}
	row := aggregatedRow{
		typ:                  requestTypeExcused,
		sortTime:             item.Request.CreatedAt,
		id:                   item.Request.ID,
		studentID:            item.Request.StudentID,
		studentName:          item.FirstName + " " + item.LastName,
		status:               item.Request.Status,
		version:              item.Request.UpdatedAt,
		urgentToday:          urgent,
		bulkEligible:         item.BulkEligible,
		bulkIneligibleReason: item.BulkIneligibleReason,
		bulkIneligibleText:   item.BulkIneligibleText,
		conflictKeys:         excusedConflictKeys(item.Request.Dates),
		currentValueChanged:  item.CurrentValueChanged,
		currentStatusByDate:  item.CurrentStatusByDate,
		data:                 toStaffExcusedRequestResponse(item),
	}
	markPast(&row, excusedScopeEnd(item.Request.Dates))
	return row
}

func careRequestUrgentToday(item *scheduleService.CareRequestReviewItem) bool {
	today := timezone.TodayDate()
	if item.Request.RequestKind == "pickup_change" {
		date, _ := item.Request.Payload["date"].(string)
		return date == today.String()
	}
	weekday := int(today.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	for _, diff := range item.Diff {
		if diff.Weekday == weekday {
			return true
		}
	}
	return false
}

// aggregatedSource pulls one type's pages on demand. It keeps the scan
// frontier (where the next DB fetch continues) separate from the consumed
// position (what the response cursor reports): the frontier may run ahead over
// rows the Go-side filters discarded, but the reported cursor must never skip
// a buffered, still-unconsumed row.
type aggregatedSource struct {
	typ      string
	fetch    func(ctx context.Context, filters modelBase.RequestQueueFilters) ([]aggregatedRow, *userService.HistoryCursor, error)
	incoming *historyCursorPayload
	scan     *historyCursorPayload
	consumed *historyCursorPayload
	buf      []aggregatedRow
	done     bool
	fetches  int
}

func (s *aggregatedSource) peek(ctx context.Context, q *aggregatedListQuery) (*aggregatedRow, error) {
	for len(s.buf) == 0 && !s.done && s.fetches < aggregatedMaxFetches {
		s.fetches++
		filters := q.queueFilters()
		filters.Limit = aggregatedFetchLimit
		if s.scan != nil {
			filters.BeforeInstant, filters.BeforeID = s.scan.UpdatedAt, s.scan.ID
		}
		rows, next, err := s.fetch(ctx, filters)
		if err != nil {
			return nil, err
		}
		for i := range rows {
			if q.matches(&rows[i]) {
				s.buf = append(s.buf, rows[i])
			}
		}
		if next == nil {
			s.done = true
		} else {
			s.scan = &historyCursorPayload{UpdatedAt: next.UpdatedAt, ID: next.ID}
		}
	}
	if len(s.buf) == 0 {
		return nil, nil
	}
	return &s.buf[0], nil
}

func (s *aggregatedSource) pop() aggregatedRow {
	row := s.buf[0]
	s.buf = s.buf[1:]
	s.consumed = &historyCursorPayload{UpdatedAt: row.sortTime, ID: row.id}
	return row
}

// hasMore reports whether this source may still hold unread rows: buffered
// ones, or DB pages the scan budget did not reach.
func (s *aggregatedSource) hasMore() bool {
	return len(s.buf) > 0 || !s.done
}

// cursorPosition is the keyset position the response cursor reports for this
// type. The bool distinguishes an omitted source from an explicit nil start
// position: the latter preserves a source with prefetched but unconsumed rows.
// Preference order: last consumed row (exact resume point); the incoming
// position while unconsumed rows sit in the buffer (never skip them); otherwise
// the scan frontier, so a page whose rows were ALL filtered still makes progress
// instead of looping the client forever.
func (s *aggregatedSource) cursorPosition() (*historyCursorPayload, bool) {
	if s.consumed != nil {
		return s.consumed, true
	}
	if len(s.buf) > 0 {
		return s.incoming, true
	}
	if !s.done && s.scan != nil {
		return s.scan, true
	}
	if s.incoming != nil {
		return s.incoming, true
	}
	return nil, false
}

// aggregatedPage merges the requested types into one keyset page. Open list
// and history walk the identical path — they differ only in which service
// method each source pulls from and which instant the rows sort on.
func (rs *Resource) aggregatedPage(ctx context.Context, q *aggregatedListQuery) (AggregatedChangeRequestPage, error) {
	if q.history {
		return rs.aggregatedPriorityPage(ctx, q)
	}
	if _, normalPhase := q.cursor[aggregatedNormalPhaseCursorKey]; normalPhase {
		normal := false
		q.urgentOnly = &normal
		page, err := rs.aggregatedPriorityPage(ctx, q)
		page.NextCursor = withNormalPhaseCursor(page.NextCursor)
		return page, err
	}
	urgent := true
	q.urgentOnly = &urgent
	page, err := rs.aggregatedPriorityPage(ctx, q)
	if err != nil || page.NextCursor != "" {
		return page, err
	}
	if len(page.Items) == q.limit {
		page.NextCursor = encodeAggregatedCursor(aggregatedCursor{aggregatedNormalPhaseCursorKey: nil})
		return page, nil
	}
	normal := *q
	normal.cursor = nil
	normal.limit = q.limit - len(page.Items)
	normalUrgent := false
	normal.urgentOnly = &normalUrgent
	normalPage, err := rs.aggregatedPriorityPage(ctx, &normal)
	if err != nil {
		return AggregatedChangeRequestPage{}, err
	}
	page.Items = append(page.Items, normalPage.Items...)
	page.NextCursor = withNormalPhaseCursor(normalPage.NextCursor)
	return page, nil
}

func (rs *Resource) aggregatedPriorityPage(ctx context.Context, q *aggregatedListQuery) (AggregatedChangeRequestPage, error) {
	sources := make([]*aggregatedSource, 0, len(q.types))
	for _, typ := range q.types {
		source := rs.sourceFor(typ, q.history)
		if pos, ok := q.cursor[typ]; ok && pos != nil {
			pin := *pos
			source.incoming = &pin
			source.scan = &pin
		}
		sources = append(sources, source)
	}

	page := AggregatedChangeRequestPage{Items: make([]AggregatedChangeRequestItem, 0, q.limit)}
	for len(page.Items) < q.limit {
		best, err := newestAggregatedSource(ctx, q, sources)
		if err != nil {
			return AggregatedChangeRequestPage{}, err
		}
		if best == nil {
			break
		}
		row := best.pop()
		page.Items = append(page.Items, newAggregatedItem(&row))
	}
	page.NextCursor = aggregatedNextCursor(sources)
	return page, nil
}

func withNormalPhaseCursor(raw string) string {
	if raw == "" {
		return ""
	}
	cursor, err := decodeAggregatedCursor(raw)
	if err != nil {
		return ""
	}
	cursor[aggregatedNormalPhaseCursorKey] = nil
	return encodeAggregatedCursor(cursor)
}

// newestAggregatedSource returns the source whose buffered head row sorts
// first across all types, or nil when every source is drained.
func newestAggregatedSource(ctx context.Context, q *aggregatedListQuery, sources []*aggregatedSource) (*aggregatedSource, error) {
	var best *aggregatedSource
	var bestRow *aggregatedRow
	for _, source := range sources {
		row, err := source.peek(ctx, q)
		if err != nil {
			return nil, err
		}
		if row != nil && (bestRow == nil || rowBefore(row, bestRow)) {
			best, bestRow = source, row
		}
	}
	return best, nil
}

// aggregatedNextCursor assembles the follow-up cursor, or "" when every source
// is exhausted.
func aggregatedNextCursor(sources []*aggregatedSource) string {
	next := make(aggregatedCursor)
	hasMore := false
	for _, source := range sources {
		if source.hasMore() {
			hasMore = true
		}
		if pos, ok := source.cursorPosition(); ok {
			next[source.typ] = pos
		}
	}
	if !hasMore {
		return ""
	}
	return encodeAggregatedCursor(next)
}

// queueFetch adapts one typed queue call to the shared source shape.
func queueFetch[T any](
	list func(ctx context.Context, filters modelBase.RequestQueueFilters) ([]T, *userService.HistoryCursor, error),
	build func(T) aggregatedRow,
) func(ctx context.Context, filters modelBase.RequestQueueFilters) ([]aggregatedRow, *userService.HistoryCursor, error) {
	return func(ctx context.Context, filters modelBase.RequestQueueFilters) ([]aggregatedRow, *userService.HistoryCursor, error) {
		items, next, err := list(ctx, filters)
		if err != nil {
			return nil, nil, err
		}
		return mapAggregatedRows(items, build), next, nil
	}
}

func (rs *Resource) sourceFor(typ string, history bool) *aggregatedSource {
	source := &aggregatedSource{typ: typ}
	switch typ {
	case requestTypeMasterData:
		if history {
			source.fetch = queueFetch(rs.MasterDataReviewService.ListHistory, masterDataHistoryRow)
		} else {
			source.fetch = queueFetch(rs.MasterDataReviewService.ListPending, masterDataPendingRow)
		}
	case requestTypeCareSchedule:
		if history {
			source.fetch = queueFetch(rs.CareRequestService.ListHistory, careHistoryRow)
		} else {
			source.fetch = queueFetch(rs.CareRequestService.ListPending, carePendingRow)
		}
	case requestTypeOffering:
		if history {
			source.fetch = queueFetch(rs.OfferingChangeService.ListHistory, offeringHistoryRow)
		} else {
			source.fetch = queueFetch(rs.OfferingChangeService.ListPending, offeringPendingRow)
		}
	case requestTypeExcused:
		if history {
			source.fetch = queueFetch(rs.ExcusedRequestService.ListHistory, excusedHistoryRow)
		} else {
			source.fetch = queueFetch(rs.ExcusedRequestService.ListPending, excusedPendingRow)
		}
	case requestTypeDirectCorrection:
		// History only — a correction has no open state. parseAggregatedListQuery
		// drops the type from the working list before it gets here.
		source.fetch = queueFetch(rs.OfferingChangeService.ListDirectCorrections, directCorrectionRow)
	}
	return source
}

func masterDataHistoryRow(item *userService.MasterDataHistoryItem) aggregatedRow {
	return aggregatedRow{
		typ:         requestTypeMasterData,
		sortTime:    item.Request.UpdatedAt,
		version:     item.Request.UpdatedAt,
		id:          item.Request.ID,
		studentID:   item.Request.StudentID,
		studentName: item.FirstName + " " + item.LastName,
		status:      item.Request.Status,
		decidedAt:   historyDecidedAt(item.Request.ReviewedAt, item.Request.UpdatedAt),
		canCorrect:  decisionIsCorrectable(item.Request.Status),
		data:        toMasterDataHistoryResponse(item),
	}
}

func careHistoryRow(item *scheduleService.CareRequestHistoryItem) aggregatedRow {
	return aggregatedRow{
		typ:         requestTypeCareSchedule,
		sortTime:    item.Request.UpdatedAt,
		version:     item.Request.UpdatedAt,
		id:          item.Request.ID,
		studentID:   item.Request.StudentID,
		studentName: item.FirstName + " " + item.LastName,
		status:      item.Request.Status,
		decidedAt:   historyDecidedAt(item.Request.ReviewedAt, item.Request.UpdatedAt),
		// A weekly plan keeps no pre-decision copy; a pickup change does.
		canCorrect: item.Request.RequestKind == "pickup_change" &&
			decisionIsCorrectable(item.Request.Status),
		data: toCareRequestHistoryResponse(item),
	}
}

func offeringHistoryRow(item *enrollmentService.OfferingChangeHistoryItem) aggregatedRow {
	return aggregatedRow{
		typ:         requestTypeOffering,
		sortTime:    item.Request.UpdatedAt,
		version:     item.Request.UpdatedAt,
		id:          item.Request.ID,
		studentID:   item.Request.StudentID,
		studentName: item.StudentName,
		status:      item.Request.Status,
		decidedAt:   historyDecidedAt(item.Request.ReviewedAt, item.Request.UpdatedAt),
		data:        toOfferingRequestHistoryResponse(item),
	}
}

func excusedHistoryRow(item *absenceService.ExcusedRequestHistoryItem) aggregatedRow {
	return aggregatedRow{
		typ:         requestTypeExcused,
		sortTime:    item.Request.UpdatedAt,
		version:     item.Request.UpdatedAt,
		id:          item.Request.ID,
		studentID:   item.Request.StudentID,
		studentName: item.FirstName + " " + item.LastName,
		status:      item.Request.Status,
		decidedAt:   historyDecidedAt(item.Request.ReviewedAt, item.Request.UpdatedAt),
		canCorrect:  decisionIsCorrectable(item.Request.Status),
		data:        toStaffExcusedHistoryResponse(item),
	}
}

// reviewAccessLevel resolves the caller's coarse reach over the queues. An
// unwired policy (bare test Resource) reports nothing rather than guessing.
func (rs *Resource) reviewAccessLevel(ctx context.Context) (string, error) {
	if rs.RequestReviewAccess == nil {
		return "", nil
	}
	return rs.RequestReviewAccess.AccessLevel(ctx, jwt.PermissionsFromCtx(ctx))
}

// Past requests (#2267, stories 14/15): a request whose whole effective scope
// lies before today changes nothing if approved. Staff still see it — an
// invisible request is one nobody ever closes — but they can only reject it or
// mark it done, and it never joins a bulk approval.
//
// Only the types with an effective scope can be past: the weekly care plan and
// a Stammdaten change apply from the decision onwards and have no end.

func excusedScopeEnd(dates []timezone.Date) timezone.Date {
	var last timezone.Date
	for _, date := range dates {
		if last.IsZero() || date.After(last) {
			last = date
		}
	}
	return last
}

func careScopeEnd(item *scheduleService.CareRequestReviewItem) timezone.Date {
	if item.Request.RequestKind != "pickup_change" {
		return timezone.Date("")
	}
	raw, _ := item.Request.Payload["date"].(string)
	date, err := timezone.ParseDate(raw)
	if err != nil {
		return timezone.Date("")
	}
	return date
}

// markPast stamps the past flag and the consequences that follow from it: no
// bulk approval, and a reason the client can render.
func markPast(row *aggregatedRow, scopeEnd timezone.Date) {
	if !userService.ParentRequestIsPast(scopeEnd, timezone.TodayDate()) {
		return
	}
	row.past = true
	row.bulkEligible = false
	row.bulkIneligibleReason = userService.BulkIneligiblePast
	row.bulkIneligibleText = "Diese Anfrage betrifft nur vergangene Tage."
}

// decorateOpenPage adds everything only the working list needs: the caller's
// review reach (so an empty list can explain itself), the child's group, and
// the Familienschutz flag. Optional dependencies stay optional — a bare test
// Resource simply omits the field.
func (rs *Resource) decorateOpenPage(ctx context.Context, page *AggregatedChangeRequestPage, types []string) error {
	access, err := rs.reviewAccessLevel(ctx)
	if err != nil {
		return err
	}
	page.ReviewAccess = access
	if err := rs.decorateConflicts(ctx, page, types); err != nil {
		return err
	}
	if rs.PersonService != nil && rs.EducationService != nil {
		if err := rs.decorateStudentGroups(ctx, page); err != nil {
			return err
		}
	}
	if rs.FamilyProtectionService != nil {
		return rs.decorateFamilyProtection(ctx, page)
	}
	return nil
}

// Conflict grouping (#2267, stories 6-10). The keys say WHAT a request would
// write, so two requests wanting opposite things for the same weekday, day,
// field or offering land in one group and the client can force a single
// result instead of letting the second decision silently overwrite the first.
//
// Derivation lives in services/users so the list and the resolve path cannot
// disagree; here we only translate each type's projection into that input.

func masterDataConflictKeys(item *userService.MasterDataReviewItem) []string {
	return userService.ParentRequestConflictKeys(userService.ParentRequestConflictInput{
		RequestType: userModels.ParentRequestTypeMasterData,
		Target:      item.Request.Target,
		Field:       item.Request.FieldKey,
	})
}

func careConflictKeys(item *scheduleService.CareRequestReviewItem) []string {
	if item.Request.RequestKind == "pickup_change" {
		date, _ := item.Request.Payload["date"].(string)
		return userService.ParentRequestConflictKeys(userService.ParentRequestConflictInput{
			RequestType: userModels.ParentRequestTypePickupChange,
			Dates:       []string{date},
		})
	}
	keys := make([]string, 0, len(item.Diff))
	for _, diff := range item.Diff {
		keys = append(keys, userService.ParentRequestConflictKeys(userService.ParentRequestConflictInput{
			RequestType: userModels.ParentRequestTypeCareSchedule,
			Weekdays:    []int{diff.Weekday},
			CareKind:    diff.CareKind,
		})...)
	}
	return keys
}

func offeringConflictKeys(item *enrollmentService.OfferingChangeView) []string {
	keys := make([]string, 0, len(item.Diff))
	for _, diff := range item.Diff {
		keys = append(keys, userService.ParentRequestConflictKeys(userService.ParentRequestConflictInput{
			RequestType: userModels.ParentRequestTypeOffering,
			OfferingID:  diff.OfferingID,
		})...)
	}
	return keys
}

func excusedConflictKeys(dates []timezone.Date) []string {
	days := make([]string, 0, len(dates))
	for _, date := range dates {
		days = append(days, date.String())
	}
	return userService.ParentRequestConflictKeys(userService.ParentRequestConflictInput{
		RequestType: userModels.ParentRequestTypeExcusedAbsence,
		Dates:       days,
	})
}

// conflictScopeLimit bounds the conflict scan. A child with more open
// requests than this is far outside anything a school produces; the group
// size is then a lower bound rather than a wrong number, and the grouping
// itself still works.
const conflictScopeLimit = 200

// decorateConflicts fills conflict_key and conflict_group_size.
//
// The scan asks the four queues for EVERY open request of the children on
// this page — deliberately not the page itself. A page is a window: two
// contradicting requests can easily land on different pages, and a group size
// counted from the window would tell staff "1" for a request that has a
// contradiction waiting one scroll away. That is the failure this whole
// feature exists to prevent, so it is worth four extra queries per page.
func (rs *Resource) decorateConflicts(ctx context.Context, page *AggregatedChangeRequestPage, types []string) error {
	studentIDs := aggregatedStudentIDs(page.Items)
	if len(studentIDs) == 0 {
		return nil
	}
	// keyed counts every open request per (student, conflict key).
	keyed := make(map[int64]map[string]int)
	count := func(studentID int64, keys []string) {
		if studentID <= 0 || len(keys) == 0 {
			return
		}
		if keyed[studentID] == nil {
			keyed[studentID] = make(map[string]int, len(keys))
		}
		for _, key := range keys {
			keyed[studentID][key]++
		}
	}
	if err := rs.scanOpenConflicts(ctx, studentIDs, types, count); err != nil {
		return err
	}
	for i := range page.Items {
		studentID, _ := strconv.ParseInt(page.Items[i].StudentID, 10, 64)
		page.Items[i].ConflictKey, page.Items[i].ConflictGroupSize =
			largestConflictGroup(keyed[studentID], page.Items[i].ConflictKeys)
	}
	return nil
}

// largestConflictGroup picks the key this item is most contended on. An item
// that shares no key with another request reports no key and a group of one,
// which is what "nothing to resolve" looks like on the wire.
func largestConflictGroup(counts map[string]int, keys []string) (string, int) {
	// Zero, not one: "no group" is absent from the wire, so a client cannot
	// mistake a lone request for a group of one and render a radio list with a
	// single option in it.
	best, bestSize := "", 0
	for _, key := range keys {
		if size := counts[key]; size > 1 && size > bestSize {
			best, bestSize = key, size
		}
	}
	return best, bestSize
}

// scanOpenConflicts walks the four open queues once each for the given
// children and reports every request's conflict keys.
// scanOpenConflicts walks the open queues the caller is actually served — a
// queue the type filter or the permission scope excluded is NOT queried, so
// the scan can never reach data the list itself refused to show. Nothing is
// lost by that: a conflict key is type-specific, so two requests can only
// share one when they come from the same queue.
func (rs *Resource) scanOpenConflicts(
	ctx context.Context,
	studentIDs []int64,
	types []string,
	report func(studentID int64, keys []string),
) error {
	filters := modelBase.RequestQueueFilters{StudentIDs: studentIDs, Limit: conflictScopeLimit}
	scans := []func() error{
		func() error {
			return scanQueueConflicts(ctx, types, requestTypeMasterData, filters, report,
				rs.MasterDataReviewService.ListPending,
				func(item *userService.MasterDataReviewItem) (int64, []string) {
					return item.Request.StudentID, masterDataConflictKeys(item)
				})
		},
		func() error {
			return scanQueueConflicts(ctx, types, requestTypeCareSchedule, filters, report,
				rs.CareRequestService.ListPending,
				func(item *scheduleService.CareRequestReviewItem) (int64, []string) {
					return item.Request.StudentID, careConflictKeys(item)
				})
		},
		func() error {
			return scanQueueConflicts(ctx, types, requestTypeOffering, filters, report,
				rs.OfferingChangeService.ListPending,
				func(item *enrollmentService.OfferingChangeView) (int64, []string) {
					return item.Request.StudentID, offeringConflictKeys(item)
				})
		},
		func() error {
			return scanQueueConflicts(ctx, types, requestTypeExcused, filters, report,
				rs.ExcusedRequestService.ListPending,
				func(item *absenceService.ExcusedRequestReviewItem) (int64, []string) {
					return item.Request.StudentID, excusedConflictKeys(item.Request.Dates)
				})
		},
	}
	for _, scan := range scans {
		if err := scan(); err != nil {
			return err
		}
	}
	return nil
}

// scanQueueConflicts is one queue's share of the scan: skip it unless the list
// is actually serving that type, then report each open request's keys.
func scanQueueConflicts[T any](
	ctx context.Context,
	types []string,
	typ string,
	filters modelBase.RequestQueueFilters,
	report func(studentID int64, keys []string),
	list func(context.Context, modelBase.RequestQueueFilters) ([]T, *userService.HistoryCursor, error),
	keys func(T) (int64, []string),
) error {
	if !slices.Contains(types, typ) {
		return nil
	}
	items, _, err := list(ctx, filters)
	if err != nil {
		return fmt.Errorf("scan %s conflicts: %w", typ, err)
	}
	for _, item := range items {
		report(keys(item))
	}
	return nil
}

// decisionIsCorrectable reports whether a history row carries a decision that
// can still be rewritten. Only a real verdict qualifies: a withdrawn request
// was never decided, a request marked done was closed BECAUSE nothing could be
// applied, and a care-end close is the school's own bookkeeping. Auto-applied
// Stammdaten rows never went through a reviewer either.
func decisionIsCorrectable(status string) bool {
	return status == "approved" || status == "rejected"
}
