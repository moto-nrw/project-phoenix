package enrollment

import (
	"context"
	"time"
)

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

func (m *Module) ApprovedOfferingChildrenForStudents(ctx context.Context, studentIDs []int64) (rows []OfferingChildFacts, err error) {
	started := time.Now()
	defer func() {
		m.observeOfferingStorage("approved_children", "query", started, int64(len(studentIDs)), int64(len(rows)), err)
	}()
	if len(studentIDs) == 0 {
		return []OfferingChildFacts{}, nil
	}
	err = m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		rows, err = m.engine.ApprovedOfferingChildrenForStudents(ctx, studentIDs)
		return err
	})
	return rows, err
}

func (m *Module) OfferingChildren(ctx context.Context, childIDs []int64) (rows []OfferingChildFacts, err error) {
	started := time.Now()
	defer func() {
		m.observeOfferingStorage("children", "query", started, int64(len(childIDs)), int64(len(rows)), err)
	}()
	if len(childIDs) == 0 {
		return []OfferingChildFacts{}, nil
	}
	err = m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		rows, err = m.engine.OfferingChildren(ctx, childIDs)
		return err
	})
	return rows, err
}
