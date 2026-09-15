package users

import "context"

// FeedbackEntryCounter is the narrow Feedback owner query the permanent
// child deletion reports in its preview. The Feedback module implements it
// without exposing persistence; the legacy composition passes it through to
// the student deletion workflow.
type FeedbackEntryCounter interface {
	CountForStudent(context.Context, int64) (int, error)
}
