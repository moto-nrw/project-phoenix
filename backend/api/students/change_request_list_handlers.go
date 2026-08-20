package students

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	absenceService "github.com/moto-nrw/project-phoenix/services/absence"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

// Aggregated Eltern request list (#2432): GET /students/change-requests
// serves all four guardian queues (Stammdaten, Betreuungszeiten, Angebote,
// Entschuldigungen) as ONE list — open or history — with server-side child
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
)

// aggregatedRequestTypeOrder is the canonical type order — it decides the
// deterministic tie-break when two rows of different types share an instant.
var aggregatedRequestTypeOrder = []string{
	requestTypeMasterData,
	requestTypeCareSchedule,
	requestTypeOffering,
	requestTypeExcused,
}

const (
	// aggregatedHistoryFetchLimit is the per-service page size of one internal
	// history fetch; larger than the response limit so name-search pages fill
	// without a fetch per row.
	aggregatedHistoryFetchLimit = 50
	// aggregatedMaxHistoryFetches bounds the DB scanning per type and request
	// when filters discard most rows. A page may come back underfilled with a
	// next_cursor; the client's "load more" continues from there.
	aggregatedMaxHistoryFetches = 4
)

var errInvalidAggregatedQuery = errors.New("invalid change request list query")

// AggregatedChangeRequestItem is one request of any of the four types. Data
// holds the unchanged per-type projection (queue shape for view=open, history
// shape for view=history), discriminated by RequestType.
type AggregatedChangeRequestItem struct {
	RequestType string `json:"request_type"`
	Data        any    `json:"data"`
}

// AggregatedChangeRequestPage is the cursor envelope of the aggregated list.
type AggregatedChangeRequestPage struct {
	Items []AggregatedChangeRequestItem `json:"items"`
	// NextCursor is absent on the last page. It is only valid for the exact
	// filter set it was produced with.
	NextCursor string `json:"next_cursor,omitempty"`
}

// aggregatedCursor maps request types to their keyset position — the last DB
// row of that type the client has consumed. Types absent from the map start
// from the top. Opaque on the wire (base64url JSON), like the per-type
// history cursor. On the open view the position instant is the row's
// created_at; the payload field name (u) just follows the shared wire shape.
type aggregatedCursor map[string]historyCursorPayload

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
		if !isAggregatedRequestType(typ) || pos.UpdatedAt.IsZero() || pos.ID <= 0 {
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
	history  bool
	search   string
	types    []string
	statuses map[string]struct{} // canonical: approved / rejected / withdrawn; empty = all
	from, to time.Time           // decided-at bounds; zero = unbounded
	limit    int
	cursor   aggregatedCursor
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

	types, err := parseAggregatedTypes(values.Get("types"))
	if err != nil {
		return q, err
	}
	q.types = types

	if err := parseAggregatedHistoryFilters(&q, values); err != nil {
		return q, err
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
	typ         string
	sortTime    time.Time
	id          int64
	studentName string
	status      string
	decidedAt   time.Time // history only
	data        any
}

// matches applies the deterministic row filters. Determinism matters: the
// cursor may advance past filtered rows, which is only safe when the same row
// is filtered identically on the next request.
func (q *aggregatedListQuery) matches(row *aggregatedRow) bool {
	if q.search != "" && !strutil.ContainsFold(row.studentName, q.search) {
		return false
	}
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

// rowBeforeCursor reports whether the row lies strictly before the keyset
// position, i.e. the client has not consumed it yet.
func rowBeforeCursor(row *aggregatedRow, pos historyCursorPayload) bool {
	if !row.sortTime.Equal(pos.UpdatedAt) {
		return row.sortTime.Before(pos.UpdatedAt)
	}
	return row.id < pos.ID
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

	var page AggregatedChangeRequestPage
	if q.history {
		page, err = rs.aggregatedHistoryPage(ctx, &q)
	} else {
		page, err = rs.aggregatedOpenPage(ctx, &q)
	}
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, page, "Change requests retrieved")
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

// aggregatedOpenPage merges the four pending queues. The per-type services
// return the full (short) scoped queue, so pagination is an in-memory keyset
// cut over the merged snapshot — same cursor semantics as the history view.
func (rs *Resource) aggregatedOpenPage(ctx context.Context, q *aggregatedListQuery) (AggregatedChangeRequestPage, error) {
	rows := make([]aggregatedRow, 0, 32)
	for _, typ := range q.types {
		typeRows, err := rs.openRowsFor(ctx, typ)
		if err != nil {
			return AggregatedChangeRequestPage{}, err
		}
		for i := range typeRows {
			if q.matches(&typeRows[i]) {
				rows = append(rows, typeRows[i])
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rowBefore(&rows[i], &rows[j]) })
	return paginateOpenRows(dropConsumedOpenRows(rows, q.cursor), q), nil
}

// dropConsumedOpenRows removes rows the client has already consumed according
// to the per-type cursor positions.
func dropConsumedOpenRows(rows []aggregatedRow, cursor aggregatedCursor) []aggregatedRow {
	if cursor == nil {
		return rows
	}
	kept := rows[:0]
	for i := range rows {
		pos, ok := cursor[rows[i].typ]
		if !ok || rowBeforeCursor(&rows[i], pos) {
			kept = append(kept, rows[i])
		}
	}
	return kept
}

// paginateOpenRows cuts the merged, filtered snapshot at the page limit and
// builds the follow-up cursor from the per-type positions of the emitted rows.
func paginateOpenRows(rows []aggregatedRow, q *aggregatedListQuery) AggregatedChangeRequestPage {
	page := AggregatedChangeRequestPage{Items: make([]AggregatedChangeRequestItem, 0, min(len(rows), q.limit))}
	next := make(aggregatedCursor, len(q.cursor)+len(q.types))
	maps.Copy(next, q.cursor)
	for i := range rows {
		if len(page.Items) == q.limit {
			page.NextCursor = encodeAggregatedCursor(next)
			return page
		}
		row := &rows[i]
		page.Items = append(page.Items, AggregatedChangeRequestItem{RequestType: row.typ, Data: row.data})
		next[row.typ] = historyCursorPayload{UpdatedAt: row.sortTime, ID: row.id}
	}
	return page
}

func (rs *Resource) openRowsFor(ctx context.Context, typ string) ([]aggregatedRow, error) {
	switch typ {
	case requestTypeMasterData:
		items, err := rs.MasterDataReviewService.ListPending(ctx)
		if err != nil {
			return nil, err
		}
		return mapAggregatedRows(items, masterDataPendingRow), nil
	case requestTypeCareSchedule:
		items, err := rs.CareRequestService.ListPending(ctx)
		if err != nil {
			return nil, err
		}
		return mapAggregatedRows(items, carePendingRow), nil
	case requestTypeOffering:
		items, err := rs.OfferingChangeService.ListPending(ctx)
		if err != nil {
			return nil, err
		}
		return mapAggregatedRows(items, offeringPendingRow), nil
	case requestTypeExcused:
		items, err := rs.ExcusedRequestService.ListPending(ctx)
		if err != nil {
			return nil, err
		}
		return mapAggregatedRows(items, excusedPendingRow), nil
	}
	return nil, errors.New("unknown aggregated request type")
}

func masterDataPendingRow(item *userService.MasterDataReviewItem) aggregatedRow {
	return aggregatedRow{
		typ:         requestTypeMasterData,
		sortTime:    item.Request.CreatedAt,
		id:          item.Request.ID,
		studentName: item.FirstName + " " + item.LastName,
		status:      item.Request.Status,
		data:        toMasterDataChangeRequestResponse(item),
	}
}

func carePendingRow(item *scheduleService.CareRequestReviewItem) aggregatedRow {
	return aggregatedRow{
		typ:         requestTypeCareSchedule,
		sortTime:    item.Request.CreatedAt,
		id:          item.Request.ID,
		studentName: item.FirstName + " " + item.LastName,
		status:      item.Request.Status,
		data:        toCareRequestResponse(item),
	}
}

func offeringPendingRow(item *enrollmentService.OfferingChangeView) aggregatedRow {
	return aggregatedRow{
		typ:         requestTypeOffering,
		sortTime:    item.Request.CreatedAt,
		id:          item.Request.ID,
		studentName: item.StudentName,
		status:      item.Request.Status,
		data:        toOfferingRequestResponse(item),
	}
}

func excusedPendingRow(item *absenceService.ExcusedRequestReviewItem) aggregatedRow {
	return aggregatedRow{
		typ:         requestTypeExcused,
		sortTime:    item.Request.CreatedAt,
		id:          item.Request.ID,
		studentName: item.FirstName + " " + item.LastName,
		status:      item.Request.Status,
		data:        toStaffExcusedRequestResponse(item),
	}
}

// aggregatedHistorySource pulls one type's history pages on demand. It keeps
// the scan frontier (where the next DB fetch continues) separate from the
// consumed position (what the response cursor reports): the frontier may run
// ahead over rows the filters discarded, but the reported cursor must never
// skip a buffered, still-unconsumed row.
type aggregatedHistorySource struct {
	typ      string
	fetch    func(ctx context.Context, before time.Time, beforeID int64, limit int) ([]aggregatedRow, *userService.HistoryCursor, error)
	incoming *historyCursorPayload
	scan     *historyCursorPayload
	consumed *historyCursorPayload
	buf      []aggregatedRow
	done     bool
	fetches  int
}

func (s *aggregatedHistorySource) peek(ctx context.Context, q *aggregatedListQuery) (*aggregatedRow, error) {
	for len(s.buf) == 0 && !s.done && s.fetches < aggregatedMaxHistoryFetches {
		s.fetches++
		var before time.Time
		var beforeID int64
		if s.scan != nil {
			before, beforeID = s.scan.UpdatedAt, s.scan.ID
		}
		rows, next, err := s.fetch(ctx, before, beforeID, aggregatedHistoryFetchLimit)
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

func (s *aggregatedHistorySource) pop() aggregatedRow {
	row := s.buf[0]
	s.buf = s.buf[1:]
	s.consumed = &historyCursorPayload{UpdatedAt: row.sortTime, ID: row.id}
	return row
}

// hasMore reports whether this source may still hold unread rows: buffered
// ones, or DB pages the scan budget did not reach.
func (s *aggregatedHistorySource) hasMore() bool {
	return len(s.buf) > 0 || !s.done
}

// cursorPosition is the keyset position the response cursor reports for this
// type. Preference order: last consumed row (exact resume point); the
// incoming position while unconsumed rows sit in the buffer (never skip
// them); otherwise the scan frontier, so a page whose rows were ALL filtered
// still makes progress instead of looping the client forever.
func (s *aggregatedHistorySource) cursorPosition() *historyCursorPayload {
	if s.consumed != nil {
		return s.consumed
	}
	if len(s.buf) > 0 {
		return s.incoming
	}
	if !s.done && s.scan != nil {
		return s.scan
	}
	return s.incoming
}

func (rs *Resource) aggregatedHistoryPage(ctx context.Context, q *aggregatedListQuery) (AggregatedChangeRequestPage, error) {
	sources := make([]*aggregatedHistorySource, 0, len(q.types))
	for _, typ := range q.types {
		source := rs.historySourceFor(typ)
		if pos, ok := q.cursor[typ]; ok {
			pin := pos
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
		page.Items = append(page.Items, AggregatedChangeRequestItem{RequestType: row.typ, Data: row.data})
	}
	page.NextCursor = aggregatedHistoryNextCursor(sources)
	return page, nil
}

// newestAggregatedSource returns the source whose buffered head row sorts
// first across all types, or nil when every source is drained.
func newestAggregatedSource(ctx context.Context, q *aggregatedListQuery, sources []*aggregatedHistorySource) (*aggregatedHistorySource, error) {
	var best *aggregatedHistorySource
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

// aggregatedHistoryNextCursor assembles the follow-up cursor, or "" when
// every source is exhausted.
func aggregatedHistoryNextCursor(sources []*aggregatedHistorySource) string {
	next := make(aggregatedCursor)
	hasMore := false
	for _, source := range sources {
		if source.hasMore() {
			hasMore = true
		}
		if pos := source.cursorPosition(); pos != nil {
			next[source.typ] = *pos
		}
	}
	if !hasMore {
		return ""
	}
	return encodeAggregatedCursor(next)
}

// historyFetch adapts one typed ListHistory call to the shared source shape.
func historyFetch[T any](
	list func(ctx context.Context, before time.Time, beforeID int64, limit int) ([]T, *userService.HistoryCursor, error),
	build func(T) aggregatedRow,
) func(ctx context.Context, before time.Time, beforeID int64, limit int) ([]aggregatedRow, *userService.HistoryCursor, error) {
	return func(ctx context.Context, before time.Time, beforeID int64, limit int) ([]aggregatedRow, *userService.HistoryCursor, error) {
		items, next, err := list(ctx, before, beforeID, limit)
		if err != nil {
			return nil, nil, err
		}
		return mapAggregatedRows(items, build), next, nil
	}
}

func (rs *Resource) historySourceFor(typ string) *aggregatedHistorySource {
	source := &aggregatedHistorySource{typ: typ}
	switch typ {
	case requestTypeMasterData:
		source.fetch = historyFetch(rs.MasterDataReviewService.ListHistory, masterDataHistoryRow)
	case requestTypeCareSchedule:
		source.fetch = historyFetch(rs.CareRequestService.ListHistory, careHistoryRow)
	case requestTypeOffering:
		source.fetch = historyFetch(rs.OfferingChangeService.ListHistory, offeringHistoryRow)
	case requestTypeExcused:
		source.fetch = historyFetch(rs.ExcusedRequestService.ListHistory, excusedHistoryRow)
	}
	return source
}

func masterDataHistoryRow(item *userService.MasterDataHistoryItem) aggregatedRow {
	return aggregatedRow{
		typ:         requestTypeMasterData,
		sortTime:    item.Request.UpdatedAt,
		id:          item.Request.ID,
		studentName: item.FirstName + " " + item.LastName,
		status:      item.Request.Status,
		decidedAt:   historyDecidedAt(item.Request.ReviewedAt, item.Request.UpdatedAt),
		data:        toMasterDataHistoryResponse(item),
	}
}

func careHistoryRow(item *scheduleService.CareRequestHistoryItem) aggregatedRow {
	return aggregatedRow{
		typ:         requestTypeCareSchedule,
		sortTime:    item.Request.UpdatedAt,
		id:          item.Request.ID,
		studentName: item.FirstName + " " + item.LastName,
		status:      item.Request.Status,
		decidedAt:   historyDecidedAt(item.Request.ReviewedAt, item.Request.UpdatedAt),
		data:        toCareRequestHistoryResponse(item),
	}
}

func offeringHistoryRow(item *enrollmentService.OfferingChangeHistoryItem) aggregatedRow {
	return aggregatedRow{
		typ:         requestTypeOffering,
		sortTime:    item.Request.UpdatedAt,
		id:          item.Request.ID,
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
		id:          item.Request.ID,
		studentName: item.FirstName + " " + item.LastName,
		status:      item.Request.Status,
		decidedAt:   historyDecidedAt(item.Request.ReviewedAt, item.Request.UpdatedAt),
		data:        toStaffExcusedHistoryResponse(item),
	}
}
