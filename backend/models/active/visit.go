package active

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/users"

	"github.com/moto-nrw/project-phoenix/models/base"
)

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

// IsActive returns whether this visit is currently active
func (v *Visit) IsActive() bool {
	return v.ExitTime == nil
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

// VisitWithStudentDisplay is the read model behind the active-group visit
// list: one open visit joined with the student's display data. Produced by
// VisitRepository.FindActiveWithStudentDisplayByGroup.
type VisitWithStudentDisplay struct {
	VisitID       int64      `bun:"visit_id"`
	StudentID     int64      `bun:"student_id"`
	PersonID      int64      `bun:"person_id"`
	ActiveGroupID int64      `bun:"active_group_id"`
	EntryTime     time.Time  `bun:"entry_time"`
	ExitTime      *time.Time `bun:"exit_time"`
	FirstName     string     `bun:"first_name"`
	LastName      string     `bun:"last_name"`
	SchoolClass   string     `bun:"school_class"`
	GroupID       *int64     `bun:"group_id"` // student's education group_id (nullable)
	OGSGroupName  string     `bun:"ogs_group_name"`
	Sick          *bool      `bun:"sick"`
	SickSince     *time.Time `bun:"sick_since"`
	Excused       *bool      `bun:"excused"`
	ExcusedSince  *time.Time `bun:"excused_since"`
	PhotoPath     *string    `bun:"photo_path"`
	CreatedAt     time.Time  `bun:"created_at"`
	UpdatedAt     time.Time  `bun:"updated_at"`
}
