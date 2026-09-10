// Package legacy adapts the retained owner services to the request-review
// projection's consumer-owned ports (#2705). It exists only because the four
// parent-request queues (Stammdaten, Betreuungszeiten, Angebote,
// Abwesenheiten), the correction log, the review policy, the student
// directory and the Familienschutz still live in legacy service packages;
// the adapters translate rows into plain records and delegate every rule
// (urgency, past scope, bulk eligibility, conflict keys, version rendering)
// to its owner, deciding nothing themselves. Delete this package with the
// last legacy source once each owner exposes the fact through its public
// capability.
package legacy

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

// ReviewPolicy is the retained review policy that explains an empty queue
// (usercontext.ParentRequestReviewPolicy).
type ReviewPolicy interface {
	AccessLevel(ctx context.Context, permissions []string) (string, error)
}

// Sources are the retained owner services the projection's ports adapt.
type Sources struct {
	MasterData   userService.MasterDataReviewService
	CareSchedule scheduleService.CareScheduleRequestService
	Offering     enrollmentService.OfferingChangeRequestService
	Excused      excusedrequests.Service
	// People and Education together resolve the child's group name; both
	// optional, the page omits the group when either is missing.
	People    userService.PersonService
	Education educationService.Service
	// FamilyProtection is optional; without it no row is marked protected.
	FamilyProtection userService.FamilyProtectionManager
	// ReviewPolicy is optional; without it the page omits review_access.
	ReviewPolicy ReviewPolicy
	// Now is the clock the projection and the row facts (urgency, past
	// scope) share; optional, defaults to time.Now.
	Now func() time.Time
}

// ErrIncompleteSources reports a missing retained queue. Missing wiring is a
// configuration error and must fail composition, not the first request.
var ErrIncompleteSources = errors.New("request review adapters are not fully configured")

// New composes the request-review projection over the retained sources. The
// four queues are required; everything else is optional.
func New(sources Sources) (requestreview.Query, error) {
	if sources.MasterData == nil || sources.CareSchedule == nil || sources.Offering == nil || sources.Excused == nil {
		return nil, ErrIncompleteSources
	}
	if sources.Now == nil {
		sources.Now = time.Now
	}
	today := func() timezone.Date { return timezone.DateFromTime(sources.Now()) }
	deps := requestreview.Dependencies{
		Queues: requestreview.Queues{
			MasterData:        masterDataQueue{service: sources.MasterData},
			CareSchedule:      careScheduleQueue{service: sources.CareSchedule, today: today},
			Offering:          offeringQueue{service: sources.Offering, today: today},
			Excused:           excusedQueue{service: sources.Excused, today: today},
			DirectCorrections: correctionLog{service: sources.Offering},
		},
		Access: access{policy: sources.ReviewPolicy},
		Today:  func() requestreview.Date { return requestreview.Date(today()) },
	}
	if sources.People != nil && sources.Education != nil {
		deps.Students = students{people: sources.People, education: sources.Education}
	}
	if sources.FamilyProtection != nil {
		deps.FamilyProtection = familyProtection{service: sources.FamilyProtection}
	}
	return requestreview.New(deps), nil
}

// access reads the caller's rights from the JWT in context and the retained
// review policy.
type access struct{ policy ReviewPolicy }

func (a access) Caller(ctx context.Context) (requestreview.Caller, error) {
	return requestreview.Caller{
		ReviewsWriteQueues: authorize.HasPermission(permissions.UsersUpdate, jwt.PermissionsFromCtx(ctx)),
	}, nil
}

// ReviewAccess resolves the caller's coarse reach over the queues. An
// unwired policy reports nothing rather than guessing.
func (a access) ReviewAccess(ctx context.Context) (string, error) {
	if a.policy == nil {
		return "", nil
	}
	return a.policy.AccessLevel(ctx, jwt.PermissionsFromCtx(ctx))
}

// students resolves each child's group name through the retained people
// and education services.
type students struct {
	people    userService.PersonService
	education educationService.Service
}

func (s students) GroupNames(ctx context.Context, studentIDs []int64) (map[int64]string, error) {
	rows, err := s.people.GetStudentsByIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	groupIDs := make([]int64, 0, len(rows))
	for _, student := range rows {
		if student != nil && student.GroupID != nil {
			groupIDs = append(groupIDs, *student.GroupID)
		}
	}
	groups, err := s.education.GetGroupsByIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(rows))
	for id, student := range rows {
		if student != nil && student.GroupID != nil && groups[*student.GroupID] != nil {
			names[id] = groups[*student.GroupID].Name
		}
	}
	return names, nil
}

type familyProtection struct {
	service userService.FamilyProtectionManager
}

func (f familyProtection) Protected(ctx context.Context, studentIDs []int64) (map[int64]bool, error) {
	current, err := f.service.Current(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	protected := make(map[int64]bool, len(current))
	for id, event := range current {
		protected[id] = event != nil && event.Enabled
	}
	return protected, nil
}

// The retained queues speak models/base filters and the users cursor; the
// two shapes are the same contract field for field.

func queueFilters(filter requestreview.QueueFilter) modelBase.RequestQueueFilters {
	filters := modelBase.RequestQueueFilters{
		UrgentOnly: filter.UrgentOnly, UrgentDate: filter.UrgentDate,
		StudentIDs: filter.StudentIDs, StudentID: filter.StudentID, Search: filter.Search,
		Limit: filter.Limit,
	}
	if filter.Before != nil {
		filters.BeforeInstant, filters.BeforeID = filter.Before.Instant, filter.Before.ID
	}
	return filters
}

func cursorOf(next *userService.HistoryCursor) *requestreview.Cursor {
	if next == nil {
		return nil
	}
	return &requestreview.Cursor{Instant: next.UpdatedAt, ID: next.ID}
}

// mapRows adapts one typed service result to the projection's row shape.
func mapRows[T any](items []T, build func(T) requestreview.Row) []requestreview.Row {
	rows := make([]requestreview.Row, 0, len(items))
	for _, item := range items {
		rows = append(rows, build(item))
	}
	return rows
}

// pendingCount mirrors the badge's retained shape: the queue is listed with
// zero-value filters and counted.
func pendingCount[T any](
	ctx context.Context,
	list func(context.Context, modelBase.RequestQueueFilters) ([]T, *userService.HistoryCursor, error),
) (int, error) {
	items, _, err := list(ctx, modelBase.RequestQueueFilters{})
	if err != nil {
		return 0, err
	}
	return len(items), nil
}

type masterDataQueue struct {
	service userService.MasterDataReviewService
}

func (q masterDataQueue) Open(ctx context.Context, filter requestreview.QueueFilter) ([]requestreview.Row, *requestreview.Cursor, error) {
	items, next, err := q.service.ListPending(ctx, queueFilters(filter))
	if err != nil {
		return nil, nil, err
	}
	return mapRows(items, masterDataPendingRow), cursorOf(next), nil
}

func (q masterDataQueue) History(ctx context.Context, filter requestreview.QueueFilter) ([]requestreview.Row, *requestreview.Cursor, error) {
	items, next, err := q.service.ListHistory(ctx, queueFilters(filter))
	if err != nil {
		return nil, nil, err
	}
	return mapRows(items, masterDataHistoryRow), cursorOf(next), nil
}

func (q masterDataQueue) OpenCount(ctx context.Context) (int, error) {
	return pendingCount(ctx, q.service.ListPending)
}

func masterDataPendingRow(item *userService.MasterDataReviewItem) requestreview.Row {
	return requestreview.Row{
		Type:                 requestreview.TypeMasterData,
		SortTime:             item.Request.CreatedAt,
		ID:                   item.Request.ID,
		StudentID:            item.Request.StudentID,
		StudentName:          item.FirstName + " " + item.LastName,
		Status:               item.Request.Status,
		Version:              userService.ParentRequestVersion(item.Request.UpdatedAt),
		BulkEligible:         item.BulkEligible,
		BulkIneligibleReason: item.BulkIneligibleReason,
		BulkIneligibleText:   item.BulkIneligibleText,
		ConflictKeys:         masterDataConflictKeys(item),
		CurrentValueChanged:  item.CurrentValueChanged,
		Data:                 ToMasterDataChangeRequestResponse(item),
	}
}

func masterDataHistoryRow(item *userService.MasterDataHistoryItem) requestreview.Row {
	return requestreview.Row{
		Type:        requestreview.TypeMasterData,
		SortTime:    item.Request.UpdatedAt,
		Version:     userService.ParentRequestVersion(item.Request.UpdatedAt),
		ID:          item.Request.ID,
		StudentID:   item.Request.StudentID,
		StudentName: item.FirstName + " " + item.LastName,
		Status:      item.Request.Status,
		DecidedAt:   HistoryDecidedAt(item.Request.ReviewedAt, item.Request.UpdatedAt),
		CanCorrect:  decisionIsCorrectable(item.Request.Status),
		Data:        ToMasterDataHistoryResponse(item),
	}
}

type careScheduleQueue struct {
	service scheduleService.CareScheduleRequestService
	today   func() timezone.Date
}

func (q careScheduleQueue) Open(ctx context.Context, filter requestreview.QueueFilter) ([]requestreview.Row, *requestreview.Cursor, error) {
	items, next, err := q.service.ListPending(ctx, queueFilters(filter))
	if err != nil {
		return nil, nil, err
	}
	today := q.today()
	return mapRows(items, func(item *scheduleService.CareRequestReviewItem) requestreview.Row {
		return carePendingRow(item, today)
	}), cursorOf(next), nil
}

func (q careScheduleQueue) History(ctx context.Context, filter requestreview.QueueFilter) ([]requestreview.Row, *requestreview.Cursor, error) {
	items, next, err := q.service.ListHistory(ctx, queueFilters(filter))
	if err != nil {
		return nil, nil, err
	}
	return mapRows(items, careHistoryRow), cursorOf(next), nil
}

func (q careScheduleQueue) OpenCount(ctx context.Context) (int, error) {
	return pendingCount(ctx, q.service.ListPending)
}

func carePendingRow(item *scheduleService.CareRequestReviewItem, today timezone.Date) requestreview.Row {
	return requestreview.Row{
		Type:                 requestreview.TypeCareSchedule,
		SortTime:             item.Request.CreatedAt,
		ID:                   item.Request.ID,
		StudentID:            item.Request.StudentID,
		StudentName:          item.FirstName + " " + item.LastName,
		Status:               item.Request.Status,
		Version:              userService.ParentRequestVersion(item.Request.UpdatedAt),
		UrgentToday:          CareRequestUrgentToday(item, today),
		Past:                 userService.ParentRequestIsPast(careScopeEnd(item), today),
		BulkIneligibleReason: userService.BulkIneligibleSingleOnly,
		BulkIneligibleText:   "Betreuungszeiten müssen einzeln geprüft werden.",
		ConflictKeys:         careConflictKeys(item),
		Data:                 ToCareRequestResponse(item),
	}
}

func careHistoryRow(item *scheduleService.CareRequestHistoryItem) requestreview.Row {
	return requestreview.Row{
		Type:        requestreview.TypeCareSchedule,
		SortTime:    item.Request.UpdatedAt,
		Version:     userService.ParentRequestVersion(item.Request.UpdatedAt),
		ID:          item.Request.ID,
		StudentID:   item.Request.StudentID,
		StudentName: item.FirstName + " " + item.LastName,
		Status:      item.Request.Status,
		DecidedAt:   HistoryDecidedAt(item.Request.ReviewedAt, item.Request.UpdatedAt),
		// A weekly plan keeps no pre-decision copy; a pickup change does.
		CanCorrect: item.Request.RequestKind == "pickup_change" &&
			decisionIsCorrectable(item.Request.Status),
		Data: ToCareRequestHistoryResponse(item),
	}
}

// CareRequestUrgentToday reports whether a care request touches today: a
// pickup change for today, or a weekly-plan change on today's weekday. It is
// the Go-side mirror of the owner queue's urgency phase.
func CareRequestUrgentToday(item *scheduleService.CareRequestReviewItem, today timezone.Date) bool {
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

// careScopeEnd is the last day a care request affects. Only a pickup change
// has one; the weekly care plan applies from the decision onwards.
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

type offeringQueue struct {
	service enrollmentService.OfferingChangeRequestService
	today   func() timezone.Date
}

func (q offeringQueue) Open(ctx context.Context, filter requestreview.QueueFilter) ([]requestreview.Row, *requestreview.Cursor, error) {
	items, next, err := q.service.ListPending(ctx, queueFilters(filter))
	if err != nil {
		return nil, nil, err
	}
	today := q.today()
	return mapRows(items, func(item *enrollmentService.OfferingChangeView) requestreview.Row {
		return offeringPendingRow(item, today)
	}), cursorOf(next), nil
}

func (q offeringQueue) History(ctx context.Context, filter requestreview.QueueFilter) ([]requestreview.Row, *requestreview.Cursor, error) {
	items, next, err := q.service.ListHistory(ctx, queueFilters(filter))
	if err != nil {
		return nil, nil, err
	}
	return mapRows(items, offeringHistoryRow), cursorOf(next), nil
}

// OpenCount uses the owner's dedicated count to avoid building every review
// diff just to render the badge.
func (q offeringQueue) OpenCount(ctx context.Context) (int, error) {
	return q.service.PendingCount(ctx)
}

func offeringPendingRow(item *enrollmentService.OfferingChangeView, today timezone.Date) requestreview.Row {
	effectiveFrom := timezone.Date(item.Request.EffectiveFrom)
	return requestreview.Row{
		Type:                 requestreview.TypeOffering,
		SortTime:             item.Request.CreatedAt,
		ID:                   item.Request.ID,
		StudentID:            item.Request.StudentID,
		StudentName:          item.StudentName,
		Status:               item.Request.Status,
		Version:              userService.ParentRequestVersion(item.Request.UpdatedAt),
		UrgentToday:          !effectiveFrom.After(today),
		Past:                 userService.ParentRequestIsPast(effectiveFrom, today),
		BulkIneligibleReason: userService.BulkIneligibleSingleOnly,
		BulkIneligibleText:   "Angebote müssen einzeln geprüft werden.",
		ConflictKeys:         offeringConflictKeys(item),
		Data:                 ToOfferingRequestResponse(item),
	}
}

func offeringHistoryRow(item *enrollmentService.OfferingChangeHistoryItem) requestreview.Row {
	return requestreview.Row{
		Type:        requestreview.TypeOffering,
		SortTime:    item.Request.UpdatedAt,
		Version:     userService.ParentRequestVersion(item.Request.UpdatedAt),
		ID:          item.Request.ID,
		StudentID:   item.Request.StudentID,
		StudentName: item.StudentName,
		Status:      item.Request.Status,
		DecidedAt:   HistoryDecidedAt(item.Request.ReviewedAt, item.Request.UpdatedAt),
		Data:        ToOfferingRequestHistoryResponse(item),
	}
}

// correctionLog serves the office's own booking corrections from the
// retained offering-change service (#2436).
type correctionLog struct {
	service enrollmentService.OfferingChangeRequestService
}

func (c correctionLog) History(ctx context.Context, filter requestreview.QueueFilter) ([]requestreview.Row, *requestreview.Cursor, error) {
	items, next, err := c.service.ListDirectCorrections(ctx, queueFilters(filter))
	if err != nil {
		return nil, nil, err
	}
	return mapRows(items, directCorrectionRow), cursorOf(next), nil
}

func directCorrectionRow(item *enrollmentService.DirectCorrectionItem) requestreview.Row {
	return requestreview.Row{
		Type:        requestreview.TypeDirectCorrection,
		SortTime:    item.Adjustment.ChangedAt,
		ID:          item.Adjustment.ID,
		StudentID:   item.Adjustment.StudentID,
		StudentName: item.StudentName,
		// A correction has no request status; the date-range filter treats the
		// moment it was applied as its "decided at".
		DecidedAt: item.Adjustment.ChangedAt,
		Data:      ToDirectCorrectionResponse(item),
	}
}

// excusedQueue adapts the Care Plan excused-absence queue (#3093), which
// speaks the owner's filter and cursor vocabulary.
type excusedQueue struct {
	service excusedrequests.Service
	today   func() timezone.Date
}

func excusedFilter(filter requestreview.QueueFilter) excusedrequests.QueueFilter {
	owner := excusedrequests.QueueFilter{
		UrgentOnly: filter.UrgentOnly, UrgentDate: filter.UrgentDate,
		StudentIDs: filter.StudentIDs, StudentID: filter.StudentID, Search: filter.Search,
		Limit: filter.Limit,
	}
	if filter.Before != nil {
		owner.BeforeInstant, owner.BeforeID = filter.Before.Instant, filter.Before.ID
	}
	return owner
}

func excusedCursorOf(next *excusedrequests.Cursor) *requestreview.Cursor {
	if next == nil {
		return nil
	}
	return &requestreview.Cursor{Instant: next.UpdatedAt, ID: next.ID}
}

func (q excusedQueue) Open(ctx context.Context, filter requestreview.QueueFilter) ([]requestreview.Row, *requestreview.Cursor, error) {
	items, next, err := q.service.ListPending(ctx, excusedFilter(filter))
	if err != nil {
		return nil, nil, err
	}
	today := q.today()
	return mapRows(items, func(item *excusedrequests.ReviewItem) requestreview.Row {
		return excusedPendingRow(item, today)
	}), excusedCursorOf(next), nil
}

func (q excusedQueue) History(ctx context.Context, filter requestreview.QueueFilter) ([]requestreview.Row, *requestreview.Cursor, error) {
	items, next, err := q.service.ListHistory(ctx, excusedFilter(filter))
	if err != nil {
		return nil, nil, err
	}
	return mapRows(items, excusedHistoryRow), excusedCursorOf(next), nil
}

// OpenCount counts the whole queue, so the filter stays at its zero value.
func (q excusedQueue) OpenCount(ctx context.Context) (int, error) {
	items, _, err := q.service.ListPending(ctx, excusedrequests.QueueFilter{})
	if err != nil {
		return 0, err
	}
	return len(items), nil
}

func excusedPendingRow(item *excusedrequests.ReviewItem, today timezone.Date) requestreview.Row {
	urgent := false
	for _, date := range item.Request.Dates {
		urgent = urgent || timezone.Date(date) == today
	}
	return requestreview.Row{
		Type:                 requestreview.TypeExcused,
		SortTime:             item.Request.CreatedAt,
		ID:                   item.Request.ID,
		StudentID:            item.Request.StudentID,
		StudentName:          item.FirstName + " " + item.LastName,
		Status:               item.Request.Status,
		Version:              userService.ParentRequestVersion(item.Request.UpdatedAt),
		UrgentToday:          urgent,
		Past:                 userService.ParentRequestIsPast(excusedScopeEnd(item.Request.Dates), today),
		BulkEligible:         item.BulkEligible,
		BulkIneligibleReason: item.BulkIneligibleReason,
		BulkIneligibleText:   item.BulkIneligibleText,
		ConflictKeys:         excusedConflictKeys(item.Request.Dates),
		CurrentValueChanged:  item.CurrentValueChanged,
		CurrentStatusByDate:  item.CurrentStatusByDate,
		Data:                 ToStaffExcusedRequestResponse(item),
	}
}

func excusedHistoryRow(item *excusedrequests.HistoryItem) requestreview.Row {
	return requestreview.Row{
		Type:        requestreview.TypeExcused,
		SortTime:    item.Request.UpdatedAt,
		Version:     userService.ParentRequestVersion(item.Request.UpdatedAt),
		ID:          item.Request.ID,
		StudentID:   item.Request.StudentID,
		StudentName: item.FirstName + " " + item.LastName,
		Status:      item.Request.Status,
		DecidedAt:   HistoryDecidedAt(item.Request.ReviewedAt, item.Request.UpdatedAt),
		CanCorrect:  decisionIsCorrectable(item.Request.Status),
		Data:        ToStaffExcusedHistoryResponse(item),
	}
}

func excusedScopeEnd(dates []excusedrequests.Date) timezone.Date {
	var last excusedrequests.Date
	for _, date := range dates {
		if last.IsZero() || date.After(last) {
			last = date
		}
	}
	return timezone.Date(last)
}

// decisionIsCorrectable reports whether a history row carries a decision that
// can still be rewritten. Only a real verdict qualifies: a withdrawn request
// was never decided, a request marked done was closed BECAUSE nothing could be
// applied, and a care-end close is the school's own bookkeeping. Auto-applied
// Stammdaten rows never went through a reviewer either.
func decisionIsCorrectable(status string) bool {
	return status == "approved" || status == "rejected"
}

// The conflict keys say WHAT a request would write. Derivation lives in
// services/users so the list and the resolve path cannot disagree; here we
// only translate each type's projection into that input.

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

func excusedConflictKeys(dates []excusedrequests.Date) []string {
	days := make([]string, 0, len(dates))
	for _, date := range dates {
		days = append(days, date.String())
	}
	return userService.ParentRequestConflictKeys(userService.ParentRequestConflictInput{
		RequestType: userModels.ParentRequestTypeExcusedAbsence,
		Dates:       days,
	})
}
