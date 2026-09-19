package domain

// StudentEnrollment uses the public profile ID, never the membership row ID.
type StudentEnrollment struct {
	StudentID     int64
	GroupID       *int64
	SchoolClass   string
	Status        string
	EnrolledFrom  string
	EnrolledUntil string
}
