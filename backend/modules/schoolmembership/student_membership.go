package schoolmembership

type StudentEnrollment struct {
	StudentID     int64
	GroupID       *int64
	SchoolClass   string
	Status        string
	EnrolledFrom  string
	EnrolledUntil string
}

// PendingWindowMove moves the enrollment window of a still pending child with
// the service window its approval planned (#6505). Each bound moves only
// while it still equals its Before value: a bound from another enrollment or
// a manual change stays. Dates are YYYY-MM-DD.
type PendingWindowMove struct {
	StudentID   int64
	FromBefore  string
	FromAfter   string
	UntilBefore string
	UntilAfter  string
}
