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
// The run joins the nightly cleanup's tenant transaction and handles one
// child per savepoint: it removes the stored photo, the photo consent and
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
	// LastCareDay is the child's last care day, empty for a graduate
	// without one.
	LastCareDay  string
	Graduated    bool
	PhotoRemoved bool
	ConsentEnded bool
}

// StudentPhotoCareEndLog appends one ended photo to the deletion log
// (audit.data_deletions) in the transaction of ctx. The composition root
// binds it; Audit Platform owns the log.
type StudentPhotoCareEndLog func(ctx context.Context, ended StudentPhotoCareEnd) error

// StudentPhotoCareEndResult reports one run: the children whose photo
// ended and those deferred because another transaction held their row.
type StudentPhotoCareEndResult struct {
	Ended    int
	Deferred int
}
