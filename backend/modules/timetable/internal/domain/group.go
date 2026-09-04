package domain

import (
	"errors"
	"time"
)

var ErrGroupNotFound = errors.New("group not found")

type Group struct {
	ID                    int64
	TenantID              int64
	CreatedAt             time.Time
	UpdatedAt             time.Time
	Name                  string
	MaxParticipants       int
	RequiredStaff         *int
	IsOpen                bool
	CategoryID            int64
	PlanningTrackID       *int64
	PlannedRoomID         *int64
	CreatedBy             *int64
	Type                  string
	EducationGroupID      *int64
	ListKind              *string
	IsTemplate            bool
	IsSystem              bool
	ArchivedAt            *time.Time
	SeriesRootID          *int64
	CalendarPeriodID      *int64
	TargetGroupType       string
	TargetGradeLevel      *int16
	TargetSchoolClass     *string
	SourceCareOfferingIDs []int64
	SourceGradeLevels     []int
	SourceSchoolClasses   []string
	Notes                 *string
	Category              *Category
}

type GroupFilter struct {
	Name        string
	CategoryID  *int64
	IsSystem    *bool
	IDs         []int64
	OrderByName bool
}

type GroupFields struct {
	Name                  string
	MaxParticipants       int
	RequiredStaff         *int
	IsOpen                bool
	CategoryID            int64
	PlanningTrackID       *int64
	PlannedRoomID         *int64
	CreatedBy             *int64
	Type                  string
	EducationGroupID      *int64
	ListKind              *string
	IsTemplate            bool
	IsSystem              bool
	ArchivedAt            *time.Time
	SeriesRootID          *int64
	CalendarPeriodID      *int64
	TargetGroupType       string
	TargetGradeLevel      *int16
	TargetSchoolClass     *string
	SourceCareOfferingIDs []int64
	SourceGradeLevels     []int
	SourceSchoolClasses   []string
	Notes                 *string
}

type GroupTarget struct {
	ID                 int64
	TenantID           int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
	ActivityGroupID    int64
	TargetGroupType    string
	TargetGradeLevel   *int16
	TargetSchoolClass  *string
	EducationGroupID   *int64
	EducationGroupName string
}

type GroupTargetFields struct {
	TargetGroupType   string
	TargetGradeLevel  *int16
	TargetSchoolClass *string
	EducationGroupID  *int64
}

type TargetStudent struct {
	ID               int64
	SchoolClass      string
	EducationGroupID *int64
	EnrolledUntil    string
}
