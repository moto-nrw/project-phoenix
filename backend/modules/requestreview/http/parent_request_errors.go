package requestreviewhttp

import (
	context "context"
)

// ParentRequestReviewAccess is the port the aggregated list uses to explain an
// empty queue and a message thread uses to decide whether a request pill may
// open its detail (#3886). Implemented by the root's ParentRequestReviewPolicy.
type ParentRequestReviewAccess interface {
	AccessLevel(ctx context.Context, permissions []string) (string, error)
	// Scope and AbsenceScope are the reach of every request kind and of the
	// sick and excused requests: school-wide, or exactly the listed groups.
	Scope(ctx context.Context, permissions []string) (schoolWide bool, groupIDs []int64, err error)
	AbsenceScope(ctx context.Context, permissions []string) (schoolWide bool, groupIDs []int64, err error)
}
