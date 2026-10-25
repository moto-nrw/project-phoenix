package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/workflows/staffoffboarding"
)

// NewStaffBirthdayWrites constructs the guarded workflow command separately
// from People Directory's read-only birthday store and staff repository.
func NewStaffBirthdayWrites(db transactionruntime.Database, membership schoolmembership.Capability) (staffoffboarding.StaffBirthdayWrites, error) {
	employment, err := workforceCompose.NewStaffEmployment(db.Pool(), nil)
	if err != nil {
		return nil, err
	}
	live, err := staffoffboarding.NewLiveStaffEmployment(membership, employment)
	if err != nil {
		return nil, membershipError("set staff birthday display opt-out", err)
	}
	return staffBirthdayWrites{live: live}, nil
}

type staffBirthdayWrites struct {
	live *staffoffboarding.LiveStaffEmployment
}

func (w staffBirthdayWrites) SetBirthdayDisplayOptOut(ctx context.Context, staffID int64, optOut bool) error {
	return membershipError("set staff birthday display opt-out", MembershipEmploymentError(w.live.SetBirthdayDisplayOptOut(ctx, staffID, optOut)))
}
