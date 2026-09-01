package users

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// Lifecycle sentinels shared by the past / mark-done / correction paths of all
// four request types (#2267). They live here, next to the coordinator, so the
// four domain services and the one handler file agree on the wire codes.
var (
	// ErrParentRequestPast means the request only covers days that have
	// passed: approving it would change nothing, so staff either reject it or
	// mark it done.
	ErrParentRequestPast = errors.New("parent requests: request only covers past days")
	// ErrParentRequestNotPast means mark-done was called on a request that
	// still covers a day in the future — that one has to be decided.
	ErrParentRequestNotPast = errors.New("parent requests: request still covers future days")
	// ErrParentRequestNotDecided means a correction was called on a request
	// that carries no decision yet.
	ErrParentRequestNotDecided = errors.New("parent requests: request is not decided")
	// ErrParentRequestCorrectionUnsupported means the decision cannot be
	// reverted safely — the type keeps no pre-state, or the live data moved on
	// since the decision. The wrapped message names the reason.
	ErrParentRequestCorrectionUnsupported = errors.New("parent requests: decision cannot be corrected")
)

// ParentRequestIsPast reports whether a request whose effective scope ends on
// scopeEnd no longer covers any day from today on. A zero scopeEnd means the
// type has no effective scope (weekly care plans, master data) and is never
// past.
func ParentRequestIsPast(scopeEnd, today timezone.Date) bool {
	if scopeEnd.IsZero() {
		return false
	}
	return scopeEnd.Before(today)
}

// Stable codes for the list's bulk_ineligible_reason. The German sentence
// travels next to them as bulk_ineligible_text; the client maps the code and
// falls back to the text for anything it does not know.
const (
	// BulkIneligiblePast — the request only covers days that have passed.
	BulkIneligiblePast = "past"
	// BulkIneligibleStale — the live value moved on after the request was filed.
	BulkIneligibleStale = "stale"
	// BulkIneligibleConflict — another entry already covers one of these days.
	BulkIneligibleConflict = "conflict"
	// BulkIneligibleChildUnavailable — the child left, graduated, or their
	// data is no longer readable.
	BulkIneligibleChildUnavailable = "child_unavailable"
	// BulkIneligibleSingleOnly — this request kind is always decided
	// individually (weekly plans, offering switches, departure modes).
	BulkIneligibleSingleOnly = "single_only"
	// BulkIneligibleAccessRevoked — the submitting guardian lost access.
	BulkIneligibleAccessRevoked = "access_revoked"
)

// CorrectionEventPayload is what a `corrected` ledger entry carries. The
// correction is a NEW entry naming what it replaced, never an edit of the old
// one — so the payload records the decision that stood before it, including
// who took it and why. That is the whole point of keeping a ledger: after a
// correction, both decisions and both reviewers are still readable.
func CorrectionEventPayload(
	approve bool,
	reason, fromStatus, toStatus string,
	priorReviewer *int64,
	priorReason *string,
) map[string]any {
	payload := map[string]any{
		"approve": approve,
		"reason":  reason,
		"from":    fromStatus,
		"to":      toStatus,
	}
	if priorReviewer != nil {
		payload["prior_reviewer"] = *priorReviewer
	}
	if priorReason != nil {
		payload["prior_reason"] = *priorReason
	}
	return payload
}
