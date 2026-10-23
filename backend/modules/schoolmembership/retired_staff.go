package schoolmembership

import (
	"context"
	"errors"
	"time"
)

// RetiredStaffErasure reports what the erasure of a retired staff member
// removed from the Membership tables (#3963). The membership stays as a
// tombstone that records of other people still name; PersonID is the person
// behind it.
type RetiredStaffErasure struct {
	StaffID                  int64
	PersonID                 int64
	Teachers                 int64
	GroupAssignments         int64
	ExternalCaregiverRecords int64
	ClassAssignments         int64
}

// EraseNextRetiredStaff erases the next staff member above afterID, in ID
// order, who left before retiredBefore and is not erased yet: it marks the
// tombstone erased and deletes the teacher record with its group assignments,
// the external caregiver record and the class assignments. found is false when none is
// due. It joins the caller's tenant transaction.
func (o *Offboarding) EraseNextRetiredStaff(ctx context.Context, retiredBefore time.Time, afterID int64) (RetiredStaffErasure, bool, error) {
	if o.eraseNext == nil {
		return RetiredStaffErasure{}, false, errors.New("school membership: retired staff erasure is not bound")
	}
	return o.eraseNext(ctx, retiredBefore, afterID)
}
