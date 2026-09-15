package workforce

import "context"

// DocumentCleanupClaim identifies one durable, token-fenced cleanup attempt.
// Stored file names and document contents never enter the job payload.
type DocumentCleanupClaim struct {
	StaffID  int64
	Token    string
	Attempts int
}

type DocumentCleanupBacklog struct {
	Pending          int
	OldestAgeSeconds float64
}

type DocumentCleanupQueue interface {
	Enqueue(context.Context, int64) error
	Claim(context.Context, int) ([]DocumentCleanupClaim, error)
	Finish(context.Context, DocumentCleanupClaim, bool) (bool, error)
	Backlog(context.Context) (DocumentCleanupBacklog, error)
}

type DocumentCleanup struct {
	enqueue func(context.Context, int64) error
	claim   func(context.Context, int) ([]DocumentCleanupClaim, error)
	finish  func(context.Context, DocumentCleanupClaim, bool) (bool, error)
	backlog func(context.Context) (DocumentCleanupBacklog, error)
}

func NewDocumentCleanup(enqueue func(context.Context, int64) error, claim func(context.Context, int) ([]DocumentCleanupClaim, error), finish func(context.Context, DocumentCleanupClaim, bool) (bool, error), backlog func(context.Context) (DocumentCleanupBacklog, error)) *DocumentCleanup {
	if enqueue == nil || claim == nil || finish == nil || backlog == nil {
		panic("workforce document cleanup: all operations are required")
	}
	return &DocumentCleanup{enqueue: enqueue, claim: claim, finish: finish, backlog: backlog}
}

func (q *DocumentCleanup) Enqueue(ctx context.Context, staffID int64) error {
	return q.enqueue(ctx, staffID)
}
func (q *DocumentCleanup) Claim(ctx context.Context, limit int) ([]DocumentCleanupClaim, error) {
	return q.claim(ctx, limit)
}
func (q *DocumentCleanup) Finish(ctx context.Context, claim DocumentCleanupClaim, success bool) (bool, error) {
	return q.finish(ctx, claim, success)
}
func (q *DocumentCleanup) Backlog(ctx context.Context) (DocumentCleanupBacklog, error) {
	return q.backlog(ctx)
}
