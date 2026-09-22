package studentpresence

// School check-in action values shared by the single and batch web endpoints.
const (
	SchoolCheckinActionIn  = "in"
	SchoolCheckinActionOut = "out"
)

// SchoolCheckinBatchItem is the per-student outcome of a batch call. OK=false
// means the id was not actionable (unknown, another tenant's student —
// invisible through the tenant filter —, or graduated); those students are
// skipped while the rest of the batch proceeds. Status carries the final
// attendance status for OK items so callers need no follow-up fetch.
type SchoolCheckinBatchItem struct {
	StudentID int64
	OK        bool
	Changed   bool
	Status    string
}

// SchoolCheckinBatchResult aggregates a batch call. Results preserve the
// caller's id order (first occurrence wins for duplicates), independent of the
// canonical write order used internally.
type SchoolCheckinBatchResult struct {
	Results   []SchoolCheckinBatchItem
	Succeeded int
	Failed    int
}

// IsSchoolCheckinNoop reports whether the requested action is already
// satisfied by the student's current attendance status ("on_yard" counts as
// present). Pure decision logic, exported for the single-student handler's
// short-circuit. The batch below deliberately does NOT use it: skipping a
// write based on a pre-read status races a concurrent transition — the
// state-checked writes themselves are the only trustworthy no-op detectors.
func IsSchoolCheckinNoop(action, currentStatus string) bool {
	alreadyPresent := currentStatus == "checked_in" || currentStatus == "on_yard"
	alreadyAbsent := currentStatus == "not_checked_in" || currentStatus == "checked_out"
	switch action {
	case SchoolCheckinActionIn:
		return alreadyPresent
	case SchoolCheckinActionOut:
		return alreadyAbsent
	default:
		return false
	}
}
