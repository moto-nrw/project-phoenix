package sample

const StaleCode = "care.announcement_stale"
const OldCode = "announcement_stale"

// FailureNotFound is a failure kind, not a wire code.
const FailureNotFound = "not_found"

type Rejection struct{}

func (Rejection) Error() string     { return "rejected" }
func (Rejection) ErrorCode() string { return UnknownCode }

// UnknownCode is not registered.
const UnknownCode = "care.not_registered"

// ConflictCode names a conflict for the client.
func ConflictCode(known bool) string {
	if known {
		return StaleCode
	}
	return "conflict"
}
