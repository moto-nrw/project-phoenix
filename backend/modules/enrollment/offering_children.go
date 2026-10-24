package enrollment

type OfferingCatalogState struct {
	Current       []*RequestChildOffering
	CapacityPeaks map[int64]int
}

// OfferingChildFacts supplies Enrollment's eligibility and student link for
// booking projections, without child names or submitted form data.
type OfferingChildFacts struct {
	ID               int64
	Status           string
	TargetGradeLevel *int16
	CreatedStudentID *int64
	MatchedStudentID *int64
}

func (c OfferingChildFacts) StudentID() int64 {
	if c.CreatedStudentID != nil {
		return *c.CreatedStudentID
	}
	if c.MatchedStudentID != nil {
		return *c.MatchedStudentID
	}
	return 0
}
