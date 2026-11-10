package schoolmembership

import "context"

type StudentEnrollment struct {
	StudentID     int64
	GroupID       *int64
	SchoolClass   string
	Status        string
	EnrolledFrom  string
	EnrolledUntil string
}

// PendingWindows moves the enrollment window of a child an approval left
// pending, for a phase edit (#6505, ADR 0200). MovePendingWindow locks only
// the child's row, takes no class gate and reports whether the child was
// still pending. It raises no Kontingentzahl: a pending start that lies ahead
// counts already, one that moves to the past no longer.
type PendingWindows interface {
	MovePendingWindow(context.Context, PendingWindowMove) (bool, error)
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
