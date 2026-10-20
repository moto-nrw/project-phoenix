package workforce

import "context"

// EmploymentReset is the accepted invitation's instruction. ActorAccountID
// accepts it; the invitation note identifies the school's prior decision.
type EmploymentReset struct {
	PreviousPersonnelNumber *string
	StaffID                 int64
	ActorAccountID          int64
	InvitationID            int64
	MasterData              bool
	Qualifications          bool
	WorkTime                bool
}

type EmploymentResetter interface {
	PrepareEmploymentReset(context.Context, EmploymentReset) (*string, error)
	ResetEmployment(context.Context, EmploymentReset) error
}
