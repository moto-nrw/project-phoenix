package domain

type TargetStudent struct {
	ID               int64
	SchoolClass      string
	EducationGroupID *int64
	EnrolledUntil    string
}
