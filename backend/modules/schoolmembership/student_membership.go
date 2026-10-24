package schoolmembership

type StudentEnrollment struct {
	StudentID     int64
	GroupID       *int64
	SchoolClass   string
	Status        string
	EnrolledFrom  string
	EnrolledUntil string
}
