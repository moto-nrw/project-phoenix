package peopledirectory

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// StudentPhotoCareEndRetention ends the photo of every child of the school in
// ctx whose care is over (#6650, Art. 17 Abs. 1 lit. a DSGVO): the photo
// serves the care, so it goes in the first nightly run after the last care
// day, without a further period. A child's care is over once its last care
// day (enrolled_until) lies before today or it graduated (alumnus).
//
// The run joins the nightly cleanup's tenant transaction and uses one
// savepoint for the batch: it removes the stored photo, the photo consent and
// who recorded it, or nothing. It never waits for a lock: a child whose row
// another transaction holds is deferred unchanged and found again in the
// next run, because the candidates come from the database every time. The
// file goes only after the commit, through the durable photo cleanup queue.
// The consent trail keeps its entries as proof of the consent; the run adds
// no withdrawal to it, because no parent withdrew anything.
type StudentPhotoCareEndRetention interface {
	EndStudentPhotosAfterCare(ctx context.Context, today calendar.Date, log StudentPhotoCareEndLog) (StudentPhotoCareEndResult, error)
}

// StudentPhotoCareEnd is one child's ended photo as the deletion log
// records it: no file name and no content.
type StudentPhotoCareEnd struct {
	TenantID  int64
	StudentID int64
	// LastCareDay is the child's last care day, nil for a graduate
	// without one.
	LastCareDay  *calendar.Date
	Graduated    bool
	PhotoRemoved bool
	ConsentEnded bool
}

// StudentPhotoCareEndLog appends the ended photos in one batch to the deletion log
// (audit.data_deletions) in the transaction of ctx. The composition root
// binds it; Audit Platform owns the log.
type StudentPhotoCareEndLog func(ctx context.Context, ended []StudentPhotoCareEnd) error

// Outcomes of one child in a run of StudentPhotoCareEndRetention.
const (
	StudentPhotoCareEnded    = "ended"
	StudentPhotoCareDeferred = "deferred"
	StudentPhotoCareKept     = "kept"
	StudentPhotoCareGone     = "gone"
)

// StudentPhotoCareEndChild is one candidate's outcome in a run: ended,
// deferred because another transaction held its row, kept because the care
// is no longer over under the lock, or gone because the child was deleted.
type StudentPhotoCareEndChild struct {
	StudentID int64
	Outcome   string
}

// StudentPhotoCareEndResult reports one run: how many photos ended, how
// many children were deferred, and every candidate's outcome, so the caller
// writes one log line per child and run.
type StudentPhotoCareEndResult struct {
	Ended    int
	Deferred int
	Children []StudentPhotoCareEndChild
}
