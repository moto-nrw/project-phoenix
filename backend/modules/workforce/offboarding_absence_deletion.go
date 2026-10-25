package workforce

import "encoding/json"

// OffboardingAbsenceDeletion is the time-tracking deletion record of one
// absence the offboarding removes, in the values the Audit Platform stores:
// the full deleted row as payload, deleted by the acting account.
type OffboardingAbsenceDeletion struct {
	StaffID   int64
	AbsenceID int64
	DeletedBy int64
	Payload   json.RawMessage
	Note      string
}

// NewOffboardingAbsenceDeletion is the deletion record of absence, removed by
// the offboarding of actorID.
func NewOffboardingAbsenceDeletion(absence StaffAbsence, actorID int64) (OffboardingAbsenceDeletion, error) {
	payload, err := json.Marshal(absence)
	if err != nil {
		return OffboardingAbsenceDeletion{}, err
	}
	return OffboardingAbsenceDeletion{StaffID: absence.StaffID, AbsenceID: absence.ID, DeletedBy: actorID, Payload: payload, Note: "Personal-Offboarding"}, nil
}
