package requestreview

import (
	"encoding/json"
	"time"
)

// The per-type "requested change" shapes. Each list item carries one of
// them under "data", discriminated by request_type: the queue shape on the
// open view, the history shape (queue shape plus decision facts) in the
// history. The decide routes answer with the same queue shapes, so the
// review card keeps working against the per-type routes.

// MasterDataChangeRequestResponse is the staff-facing projection of one parent
// Stammdaten change request in the review queue.
type MasterDataChangeRequestResponse struct {
	ID         string          `json:"id"`
	StudentID  string          `json:"student_id"`
	FirstName  string          `json:"first_name"`
	LastName   string          `json:"last_name"`
	Target     string          `json:"target"`
	FieldKey   string          `json:"field_key"`
	OldValue   json.RawMessage `json:"old_value,omitempty"`
	NewValue   json.RawMessage `json:"new_value"`
	Status     string          `json:"status"`
	CreatedAt  time.Time       `json:"created_at"`
	ReviewedAt *time.Time      `json:"reviewed_at,omitempty"`
}

// MasterDataChangeRequestHistoryResponse extends the queue projection with the
// decision facts for the staff history.
type MasterDataChangeRequestHistoryResponse struct {
	MasterDataChangeRequestResponse
	DecidedAt     time.Time `json:"decided_at"`
	DecidedByName string    `json:"decided_by_name,omitempty"`
	ReviewReason  *string   `json:"review_reason,omitempty"`
}

// CareRequestResponse is the staff-facing projection of one parent
// care-schedule change request in the review queue, including the live
// "current → requested" weekly diff.
type CareRequestResponse struct {
	ID              string                    `json:"id"`
	StudentID       string                    `json:"student_id"`
	FirstName       string                    `json:"first_name"`
	LastName        string                    `json:"last_name"`
	Status          string                    `json:"status"`
	RequestKind     string                    `json:"request_kind"`
	Diff            []CareRequestDiffResponse `json:"diff"`
	RequestReason   *string                   `json:"request_reason,omitempty"`
	DecisionReason  *string                   `json:"decision_reason,omitempty"`
	CreatedAt       time.Time                 `json:"created_at"`
	ReviewedAt      *time.Time                `json:"reviewed_at,omitempty"`
	AffectedBlocks  []AffectedCareBlock       `json:"affected_blocks"`
	ImpactAvailable bool                      `json:"impact_available"`
	ImpactToken     string                    `json:"impact_token"`
}

// AffectedCareBlock is one timetable block a care-schedule change touches.
type AffectedCareBlock struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

// CareRequestDiffResponse mirrors the request-diff wire shape the messaging
// thread page used, so the frontend's RequestDiffPanel renders it unchanged.
type CareRequestDiffResponse struct {
	Label    string   `json:"label"`
	Old      string   `json:"old"`
	New      string   `json:"new"`
	Weekday  int      `json:"weekday,omitempty"`
	CareKind string   `json:"care_kind,omitempty"`
	OldModes []string `json:"old_modes,omitempty"`
	NewMode  string   `json:"new_mode,omitempty"`
}

// CareRequestHistoryResponse is one decided care-schedule request. It never
// carries a recomputed live diff — current data has moved on since the
// decision. Diff replays the frozen decision snapshot (ADR 0002, #2430);
// rows without one (withdrawn, pre-snapshot) fall back to the payload-derived
// requested summary (each entry's old side is empty).
type CareRequestHistoryResponse struct {
	ID             string                    `json:"id"`
	StudentID      string                    `json:"student_id"`
	FirstName      string                    `json:"first_name"`
	LastName       string                    `json:"last_name"`
	Status         string                    `json:"status"`
	RequestKind    string                    `json:"request_kind"`
	Requested      []CareRequestDiffResponse `json:"requested"`
	Diff           []CareRequestDiffResponse `json:"diff,omitempty"`
	DecisionReason *string                   `json:"decision_reason,omitempty"`
	CreatedAt      time.Time                 `json:"created_at"`
	DecidedAt      time.Time                 `json:"decided_at"`
	DecidedByName  string                    `json:"decided_by_name,omitempty"`
}

// OfferingRequestResponse is the staff-facing projection of one parent
// offering-change request in the review queue, with the live
// "current → requested" diff (#1665).
type OfferingRequestResponse struct {
	ID          string `json:"id"`
	StudentID   string `json:"student_id"`
	StudentName string `json:"student_name"`
	Status      string `json:"status"`
	// EffectiveFrom is the date the switch would take effect (YYYY-MM-DD).
	EffectiveFrom string `json:"effective_from"`
	// EarliestEffectiveFrom / LatestEffectiveFrom bound the date staff may
	// confirm the switch for (#2484), so the review card cannot offer a date the
	// approval refuses. Omitted when the care period could not be resolved.
	EarliestEffectiveFrom string `json:"earliest_effective_from,omitempty"`
	LatestEffectiveFrom   string `json:"latest_effective_from,omitempty"`
	// RequestedEffectiveFrom is the date the family asked for, sent only when it
	// is not the date the queue offers — a request whose date passed while it
	// waited applies at the earliest date left instead (#2484).
	RequestedEffectiveFrom string                        `json:"requested_effective_from,omitempty"`
	Note                   string                        `json:"note,omitempty"`
	Diff                   []OfferingRequestDiffResponse `json:"diff"`
	// Unchanged lists the bookings the request leaves as they are, so the review
	// card shows the child's complete picture, not only the changed lines (#2434).
	Unchanged []OfferingRequestUnchangedResponse `json:"unchanged,omitempty"`
	// FullWithdrawal marks a Komplett-Abmeldung: approving would leave the child
	// without any offering at all (#2434).
	FullWithdrawal bool       `json:"full_withdrawal,omitempty"`
	Reason         *string    `json:"reason,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	ReviewedAt     *time.Time `json:"reviewed_at,omitempty"`
}

// OfferingRequestDiffResponse is one German-localized diff line for the
// German-only staff portal.
type OfferingRequestDiffResponse struct {
	OfferingID string `json:"offering_id"`
	Label      string `json:"label"`
	Old        string `json:"old"`
	New        string `json:"new"`
	// Automatic marks a line whose NEW side contains days a Mitbuchungs-Regel
	// (or the required lunch) added rather than the parents (#2365).
	Automatic bool `json:"automatic,omitempty"`
	// AutomaticDays is the German day list of that automatic share ("Do, Fr").
	AutomaticDays string `json:"automatic_days,omitempty"`
	// RuleDays is the part attributed to TriggerNames. Required-lunch days are
	// excluded so the explanation does not ascribe them to a Mitbuchungs-Regel.
	RuleDays string `json:"rule_days,omitempty"`
	// NewWhenExcluded is the materialized NEW side after this line's
	// Mitbuchungs-Regel is suppressed. Manual and required-lunch days remain.
	NewWhenExcluded string `json:"new_when_excluded,omitempty"`
	// TriggerIDs / TriggerNames identify the selected offerings whose rule
	// produced the automatic share. TriggerIDs lets the review card grey out
	// dependent lines while staff untick an override (#2370).
	TriggerIDs   []string `json:"trigger_ids,omitempty"`
	TriggerNames []string `json:"trigger_names,omitempty"`
	// Optoutable marks a rule-triggered line staff may exclude per request.
	Optoutable bool `json:"optoutable,omitempty"`
	// IsCourse marks a line about a Kurs (an AG reached through a care
	// offering, #3075), so the card can say what kind of request this is.
	IsCourse bool `json:"is_course,omitempty"`
}

// OfferingRequestUnchangedResponse is one booking the request does not touch.
type OfferingRequestUnchangedResponse struct {
	OfferingID string `json:"offering_id"`
	Label      string `json:"label"`
	Days       string `json:"days"`
}

// OfferingRequestHistoryResponse extends the queue projection with the
// decision facts. Its diff comes from the frozen decision snapshot.
type OfferingRequestHistoryResponse struct {
	OfferingRequestResponse
	DecidedAt     time.Time                          `json:"decided_at"`
	DecidedByName string                             `json:"decided_by_name,omitempty"`
	Requested     []OfferingRequestRequestedResponse `json:"requested,omitempty"`
}

// OfferingRequestRequestedResponse is a payload-derived recap for history
// rows without a frozen decision snapshot, such as withdrawn requests.
type OfferingRequestRequestedResponse struct {
	OfferingID string `json:"offering_id"`
	Label      string `json:"label"`
	New        string `json:"new"`
}

// StaffExcusedRequestResponse is the legacy-named staff projection of one
// parent absence approval request in the review queue.
type StaffExcusedRequestResponse struct {
	ID            string     `json:"id"`
	StudentID     string     `json:"student_id"`
	FirstName     string     `json:"first_name"`
	LastName      string     `json:"last_name"`
	AbsenceStatus string     `json:"absence_status"`
	Status        string     `json:"status"`
	Dates         []string   `json:"dates"`
	Note          string     `json:"note"`
	Reason        *string    `json:"reason,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	ReviewedAt    *time.Time `json:"reviewed_at,omitempty"`
}

// StaffExcusedRequestHistoryResponse extends the queue projection with the
// decision facts for the staff history.
type StaffExcusedRequestHistoryResponse struct {
	StaffExcusedRequestResponse
	DecidedAt     time.Time `json:"decided_at"`
	DecidedByName string    `json:"decided_by_name,omitempty"`
}

// DirectCorrectionResponse is one admin correction to a child's bookings as
// the central history renders it (#2436). It deliberately carries no status
// and no decision fields: a correction is not a request, it has no open state
// and nothing was decided about it.
type DirectCorrectionResponse struct {
	ID          string `json:"id"`
	StudentID   string `json:"student_id"`
	StudentName string `json:"student_name"`
	// ChangedAt is when the office applied the correction.
	ChangedAt time.Time `json:"changed_at"`
	// ChangedByName is the actor snapshot taken at write time, "Unbekannt"
	// when the account is gone.
	ChangedByName string                        `json:"changed_by_name"`
	Reason        string                        `json:"reason"`
	Diff          []OfferingRequestDiffResponse `json:"diff"`
}
