package studentdeletion

import "strconv"

// The deletion types of a child's deletion log entry, as the Audit Platform
// stores them: a deletion a staff member confirmed, and the automatic
// deletion of the nightly retention run (#3962).
const (
	DeletionTypeManual    = "manual"
	DeletionTypeRetention = "student_retention"
)

// DeletionAudit is the deletion log entry and the tombstone of one deleted
// child, in the values the Audit Platform stores.
type DeletionAudit struct {
	TenantID       int64
	StudentID      int64
	DeletionType   string
	RecordsDeleted int
	DeletedBy      string
	// ActorAccountID is nil for the nightly retention run: nobody confirmed
	// the deletion.
	ActorAccountID *int64
	Reason         string
	Counts         Counts
}

// NewDeletionAudit is the audit of the deletion result, attributed to actor.
// Actor account 0 is the nightly retention run (#3962): no actor, written by
// system.
func NewDeletionAudit(actor Actor, result Result) DeletionAudit {
	audit := DeletionAudit{
		TenantID: actor.TenantID, StudentID: result.StudentID,
		DeletionType: DeletionTypeManual, RecordsDeleted: result.Counts.Total() + result.PrimaryRowsDeleted,
		DeletedBy: "account:" + strconv.FormatInt(actor.AccountID, 10), ActorAccountID: &actor.AccountID,
		Reason: result.Reason, Counts: result.Counts,
	}
	if actor.AccountID == 0 {
		audit.DeletionType, audit.DeletedBy, audit.ActorAccountID = DeletionTypeRetention, "system", nil
	}
	return audit
}

// EachMetadata hands set the metadata of the deletion log entry: the mark of
// a student deletion and the deleted rows per kind.
func (a DeletionAudit) EachMetadata(set func(key string, value any)) {
	set("student_deletion", true)
	set("counts", a.Counts)
}
