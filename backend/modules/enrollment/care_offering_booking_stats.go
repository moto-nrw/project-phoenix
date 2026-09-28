package enrollment

// CareOfferingBookingStat is one offering's admin-facing booking summary as
// the enrollment routes render it.
type CareOfferingBookingStat struct {
	OfferingID        int64
	Capacity          *int
	Booked            int
	GradeLevels       map[int]int
	UnknownGradeCount int
}
