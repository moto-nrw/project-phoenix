package users

import (
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
)

// CompanionStrandingBatch collects the "would dropping this edge strand the far
// child" verdicts of a COORDINATED multi-child write, so they can be decided
// once against the final state of every child in the batch instead of against
// the intermediate state each individual write happens to see.
//
// Why this exists (#1694): the enrollment change-request approval applies one
// approved child after the other, but a single approval is one atomic edit of
// the whole family. Two children linked only to each other, both dropping the
// accompanied mode in the same approval, are a VALID edit — after it, neither
// claims "Anderes Kind" and neither needs a "mit wem" detail. Evaluated
// per-write, the first child's write sees the second one still carrying its old
// accompanied plan, no note and no other link, and refuses the whole approval
// with ErrCompanionWouldLoseDeparture. Ordering the writes cannot fix it: the
// same argument refuses whichever child goes first.
//
// A batch defers those verdicts. The write path records them here instead of
// refusing, and the coordinating service decides them once at the end via
// StudentRepository.VerifyCompanionStrandingBatch, when every plan change and
// every edge deletion of the batch is already applied. The verdict is then the
// same one a single-child write would reach against the final state: a child
// left with an accompanied day, no note and no remaining link on that day still
// fails the batch (and rolls the whole approval back), a child whose own plan
// dropped that day passes.
//
// Scope is one transaction — the batch lives in the context of the coordinating
// write and is never shared across requests or goroutines.
// CompanionStrandingBatch is the owner's batch under the name the retained
// callers already use (#3349): opening a scope here is opening the owner's.
type CompanionStrandingBatch = departure.StrandingBatch
