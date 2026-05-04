package active

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/users"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// tableActiveVisits is the schema-qualified table name for visits
const tableActiveVisits = "active.visits"

// Visit represents a student visit to an active group
type Visit struct {
	base.Model `bun:"schema:active,table:visits"`
	base.TenantModel
	StudentID     int64      `bun:"student_id,notnull" json:"student_id"`
	ActiveGroupID int64      `bun:"active_group_id,notnull" json:"active_group_id"`
	EntryTime     time.Time  `bun:"entry_time,notnull" json:"entry_time"`
	ExitTime      *time.Time `bun:"exit_time" json:"exit_time,omitempty"`

	// Relations - these would be populated when using the ORM's relations
	Student     *users.Student `bun:"rel:belongs-to,join:student_id=id" json:"student,omitempty"`
	ActiveGroup *Group         `bun:"rel:belongs-to,join:active_group_id=id" json:"active_group,omitempty"`
}

func (v *Visit) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableActiveVisits)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableActiveVisits)
	}
	if q, ok := query.(*bun.InsertQuery); ok {
		q.ModelTableExpr(tableActiveVisits)
	}
	return nil
}

// GetID returns the entity's ID
func (v *Visit) GetID() interface{} {
	return v.ID
}

// GetCreatedAt returns the creation timestamp
func (v *Visit) GetCreatedAt() time.Time {
	return v.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (v *Visit) GetUpdatedAt() time.Time {
	return v.UpdatedAt
}

// TableName returns the database table name
func (v *Visit) TableName() string {
	return tableActiveVisits
}

// Validate ensures active visit data is valid
func (v *Visit) Validate() error {
	if v.StudentID <= 0 {
		return errors.New("student ID is required")
	}

	if v.ActiveGroupID <= 0 {
		return errors.New("active group ID is required")
	}

	if v.EntryTime.IsZero() {
		return errors.New("entry time is required")
	}

	if v.ExitTime != nil && v.EntryTime.After(*v.ExitTime) {
		return errors.New("entry time must be before exit time")
	}

	return nil
}

// VisitGroupNames holds the activity and room names from a visit for indicator matching.
type VisitGroupNames struct {
	StudentID         int64
	ActivityGroupName string
	RoomName          string
}

// StudentInRoomVisit is a denormalised row representing one student currently
// checked-in to an active group that takes place in a given room. Returned by
// VisitRepository.ListActiveByRoomID. EducationGroupID is the student's
// education group (users.students.group_id); the handler uses it to drive
// GDPR redaction via api/common.StudentAccessContext.HasFullAccessByGroupID.
//
// EducationGroupName is the human-readable Stammgruppe name (e.g.
// "Sternengruppe"). The room-detail "Kinder im Raum" UI surfaces this so the
// label matches the convention on the Kindersuche page (Gruppe = Stammgruppe,
// not the active activity). Empty when the student has no education group
// assignment, in which case the handler omits the field from the response.
type StudentInRoomVisit struct {
	StudentID          int64
	FirstName          string
	LastName           string
	EducationGroupID   *int64
	EducationGroupName string
	ActiveGroupID      int64
	EntryTime          time.Time
}

// IsActive returns whether this visit is currently active
func (v *Visit) IsActive() bool {
	return v.ExitTime == nil
}

// EndVisit sets the exit time to the current time
func (v *Visit) EndVisit() {
	now := time.Now()
	v.ExitTime = &now
}

// SetExitTime explicitly sets the exit time
func (v *Visit) SetExitTime(exitTime time.Time) error {
	if v.EntryTime.After(exitTime) {
		return errors.New("exit time cannot be before entry time")
	}
	v.ExitTime = &exitTime
	return nil
}

// GetDuration returns the duration of the visit
func (v *Visit) GetDuration() time.Duration {
	if v.ExitTime == nil {
		return time.Since(v.EntryTime)
	}
	return v.ExitTime.Sub(v.EntryTime)
}
