package offeringcontract

// OfferingChangeDate is the consumer's ISO calendar day, never an instant.
type OfferingChangeDate string

// Offering-change-request lifecycle. A guardian submits a pending row; staff
// approve it (which applies the switch on its effective date) or reject it; the
// submitting guardian may withdraw it while it is still pending.
const (
	OfferingChangeStatusPending   = "pending"
	OfferingChangeStatusApproved  = "approved"
	OfferingChangeStatusRejected  = "rejected"
	OfferingChangeStatusWithdrawn = "withdrawn"
	// OfferingChangeStatusDone closes a request that only covered days already
	// gone: nothing to apply, and "abgelehnt" would misstate what happened.
	OfferingChangeStatusDone = "done"
	// OfferingChangeStatusCareEnded closes an open request whose child left
	// the OGS before anybody decided it (#2487).
	OfferingChangeStatusCareEnded = "care_ended"
)

// OfferingChangeDecisionSnapshot is the frozen review diff stored on a decided
// request (see the DecisionSnapshot field).
type OfferingChangeDecisionSnapshot struct {
	Diff []OfferingChangeSnapshotEntry `json:"diff"`
	// OverriddenOfferings lists the Mitbuchungs-Regel targets staff excluded
	// for this one request at approval time (#2370).
	OverriddenOfferings []OfferingChangeSnapshotOffering `json:"overridden_offerings,omitempty"`
}

// OfferingChangeSnapshotEntry is one frozen diff line.
type OfferingChangeSnapshotEntry struct {
	OfferingID int64    `json:"offering_id"`
	Label      string   `json:"label"`
	OldState   string   `json:"old_state"`
	OldDays    []string `json:"old_days,omitempty"`
	NewState   string   `json:"new_state"`
	NewDays    []string `json:"new_days,omitempty"`
	// NewAutomaticDays is the share of NewDays a Mitbuchungs-Regel (or the
	// required-lunch derivation) added rather than the parents.
	NewAutomaticDays []string `json:"new_automatic_days,omitempty"`
	// NewRuleDays is the subset added specifically by a Mitbuchungs-Regel.
	NewRuleDays []string `json:"new_rule_days,omitempty"`
	// AutoTriggerNames names the selected offerings that triggered the
	// automatic share; empty for the required-lunch derivation.
	AutoTriggerNames []string `json:"auto_trigger_names,omitempty"`
	// IsCourse preserves the staff-visible course marker on a decided request.
	IsCourse bool `json:"is_course,omitempty"`
}

// OfferingChangeSnapshotOffering names one overridden rule target.
type OfferingChangeSnapshotOffering struct {
	OfferingID int64  `json:"offering_id"`
	Name       string `json:"name"`
}
