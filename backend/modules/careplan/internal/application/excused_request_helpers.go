package application

import (
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// probeLimit asks the store for one row more than the caller wants, so a
// present extra row proves an older page exists. An unbounded page stays
// unbounded.
func probeLimit(filter careplan.RequestQueueFilter) careplan.RequestQueueFilter {
	if filter.Limit > 0 {
		filter.Limit++
	}
	return filter
}

// nextCursor trims the probe row off a page and returns the position to
// resume from, or nil on the last page.
func nextCursor[T any](rows []T, limit int, key func(T) (time.Time, int64)) ([]T, *careplan.RequestCursor) {
	if limit <= 0 || len(rows) <= limit {
		return rows, nil
	}
	rows = rows[:limit]
	instant, id := key(rows[len(rows)-1])
	return rows, &careplan.RequestCursor{UpdatedAt: instant, ID: id}
}

// reviewerDisplayName resolves a nullable reviewer account id to a display
// name. A decided row whose reviewer account was deleted still shows up, as
// "Unbekannt", instead of losing the fact that somebody decided it.
func reviewerDisplayName(reviewers map[int64]careplan.PersonName, reviewedBy *int64) string {
	if reviewedBy == nil || *reviewedBy <= 0 {
		return ""
	}
	if p, ok := reviewers[*reviewedBy]; ok {
		return strings.TrimSpace(p.FirstName + " " + p.LastName)
	}
	return "Unbekannt"
}

func requestExtendsBeyondCare(req *careplan.ExcusedAbsenceRequest, student *careplan.ReviewStudent) bool {
	if req == nil || student == nil || student.EnrolledUntil.IsZero() {
		return false
	}
	for _, date := range req.Dates {
		if date.After(student.EnrolledUntil) {
			return true
		}
	}
	return false
}

// parentRequestIsPast reports whether a request whose scope ends on scopeEnd
// no longer covers any day from today on. A zero scopeEnd is never past.
func parentRequestIsPast(scopeEnd, today careplan.Date) bool {
	return !scopeEnd.IsZero() && scopeEnd.Before(today)
}

// absenceBulkEligible reports whether every requested day is today or later.
func absenceBulkEligible(dates []careplan.Date, today careplan.Date) bool {
	if len(dates) == 0 {
		return false
	}
	for _, date := range dates {
		if date.Before(today) {
			return false
		}
	}
	return true
}

// excusedScopeEnd is the last day this request covers.
func excusedScopeEnd(req *careplan.ExcusedAbsenceRequest) careplan.Date {
	var last careplan.Date
	for _, date := range req.Dates {
		if last.IsZero() || date.After(last) {
			last = date
		}
	}
	return last
}

func isCorrectableExcusedStatus(status string) bool {
	return status == careplan.ExcusedRequestStatusApproved || status == careplan.ExcusedRequestStatusRejected
}

func absenceRequestCopy(absenceStatus string) (created, confirmed, requestType string) {
	if absenceStatus == careplan.StudentStatusDaySick {
		return sickRequestCreatedBody, sickRequestConfirmedBody, careplan.ParentMessageRequestSick
	}
	return excusedRequestCreatedBody, excusedRequestConfirmedBody, careplan.ParentMessageRequestExcused
}

func absenceRequestRejectedBody(absenceStatus string) string {
	if absenceStatus == careplan.StudentStatusDaySick {
		return sickRequestRejectedBody
	}
	return excusedRequestRejectedBody
}

// correctedAbsencePill names the new outcome for the family. The wording says
// the decision changed, because "bestätigt" alone after a rejection reads
// like a duplicate message rather than a correction.
func correctedAbsencePill(absenceStatus string, approve bool) (string, string) {
	if approve {
		_, confirmed, _ := absenceRequestCopy(absenceStatus)
		return "Entscheidung geändert: " + confirmed, careplan.ParentMessageRequestStatusDone
	}
	return "Entscheidung geändert: " + absenceRequestRejectedBody(absenceStatus), careplan.ParentMessageRequestStatusReject
}

// coGuardianNoticeBody names what changed and when, and nothing else.
func coGuardianNoticeBody(req *careplan.ExcusedAbsenceRequest) string {
	kind := "Abmeldung"
	if req.AbsenceStatus == careplan.StudentStatusDaySick {
		kind = "Krankmeldung"
	}
	return "Betreuungsstand geändert: " + kind + " " + absenceDateRangeLabel(req.Dates)
}

// absenceDateRangeLabel renders one day as a date and several as a range.
func absenceDateRangeLabel(dates []careplan.Date) string {
	if len(dates) == 0 {
		return ""
	}
	first := germanDate(dates[0])
	if len(dates) == 1 {
		return first
	}
	return first + " bis " + germanDate(dates[len(dates)-1])
}

// germanDate renders a canonical date as DD.MM.YYYY; an unparsable value is
// shown as stored rather than hidden.
func germanDate(date careplan.Date) string {
	parsed, err := time.Parse(careplan.DateLayout, date.String())
	if err != nil {
		return date.String()
	}
	return parsed.Format("02.01.2006")
}

// correctionEventPayload is what a corrected ledger entry carries: the new
// verdict and the decision that stood before it, including who took it and
// why.
func correctionEventPayload(approve bool, reason, fromStatus, toStatus string, priorReviewer *int64, priorReason *string) map[string]any {
	payload := map[string]any{"approve": approve, "reason": reason, "from": fromStatus, "to": toStatus}
	if priorReviewer != nil {
		payload["prior_reviewer"] = *priorReviewer
	}
	if priorReason != nil {
		payload["prior_reason"] = *priorReason
	}
	return payload
}

// Before absence_status existed, every row represented an excused absence.
// The migration backfills persisted rows; this fallback keeps zero-value
// test doubles compatible too.
func normalizedAbsenceRequestStatus(absenceStatus string) string {
	if absenceStatus == "" {
		return careplan.StudentStatusDayExcused
	}
	return absenceStatus
}

// sameDateSet reports whether two date slices contain exactly the same days.
// Both are stored deduped and sorted, so a positional comparison suffices.
func sameDateSet(a, b []careplan.Date) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// datesIntersect reports whether the two date slices share at least one day.
func datesIntersect(a, b []careplan.Date) bool {
	set := make(map[careplan.Date]struct{}, len(a))
	for _, d := range a {
		set[d] = struct{}{}
	}
	for _, d := range b {
		if _, ok := set[d]; ok {
			return true
		}
	}
	return false
}

// dedupeSortedDates removes duplicate dates and returns them ascending, so
// the stored payload is canonical regardless of client ordering.
func dedupeSortedDates(dates []careplan.Date) []careplan.Date {
	seen := make(map[careplan.Date]struct{}, len(dates))
	out := make([]careplan.Date, 0, len(dates))
	for _, d := range dates {
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out
}

func containsDate(dates []careplan.Date, needle careplan.Date) bool {
	for _, d := range dates {
		if d == needle {
			return true
		}
	}
	return false
}
