package enrollment

import "context"

func (m *Module) OfferingGradeCounts(ctx context.Context, ids []int64, from, until Date) (rows []*OfferingGradeCount, err error) {
	if len(ids) == 0 {
		return []*OfferingGradeCount{}, nil
	}
	err = m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		rows, err = m.engine.OfferingGradeCounts(ctx, ids, from, until)
		return err
	})
	return rows, err
}

func (m *Module) MaterializableOfferingCount(ctx context.Context, id int64, today Date) (count int, err error) {
	err = m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		count, err = m.engine.MaterializableOfferingCount(ctx, id, today)
		return err
	})
	return count, err
}

// OfferingCapacityPeaks counts peak occupancy per offering in one query.
// Offerings with no overlapping intervals are absent from the result.
func (m *Module) OfferingCapacityPeaks(ctx context.Context, offeringIDs []int64, from, until Date) (counts map[int64]int, err error) {
	if len(offeringIDs) == 0 {
		return map[int64]int{}, nil
	}
	err = m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		counts, err = m.engine.OfferingCapacityPeaks(ctx, offeringIDs, from, until)
		return err
	})
	return counts, err
}

// OfferingCapacityPeak counts distinct non-terminal children at the busiest
// interval boundary in [from, until), excluding the supplied children.
func (m *Module) OfferingCapacityPeak(ctx context.Context, offeringID int64, excludedChildren []int64, from, until Date) (count int, err error) {
	err = m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		count, err = m.engine.OfferingCapacityPeak(ctx, offeringID, excludedChildren, from, until)
		return err
	})
	return count, err
}

// RequestChildOfferingHistory returns all selection intervals ordered by ID.
func (m *Module) RequestChildOfferingHistory(ctx context.Context, childID int64) (rows []*RequestChildOffering, err error) {
	err = m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		rows, err = m.engine.RequestChildOfferingHistory(ctx, childID)
		return err
	})
	return rows, err
}

func (m *Module) InsertRequestChildOffering(ctx context.Context, selection *RequestChildOffering) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error { return m.engine.InsertRequestChildOffering(ctx, selection) })
}

func (m *Module) ReplaceRequestChildOfferings(ctx context.Context, childID int64, selections []*RequestChildOffering) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		return m.engine.ReplaceRequestChildOfferings(ctx, childID, selections)
	})
}

func (m *Module) ScheduleRequestChildOfferings(ctx context.Context, childID int64, effectiveFrom Date, selections []*RequestChildOffering) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		return m.engine.ScheduleRequestChildOfferings(ctx, childID, effectiveFrom, selections)
	})
}

// RequestChildOfferingsAtDate returns active selections, or the first upcoming
// interval before the phase starts. Gaps during the phase remain empty.
func (m *Module) RequestChildOfferingsAtDate(ctx context.Context, childID int64, onDate Date) (rows []*RequestChildOffering, err error) {
	err = m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		rows, err = m.engine.RequestChildOfferingsAtDate(ctx, childID, onDate)
		return err
	})
	return rows, err
}

// RequestChildOfferingsForChildrenAtDate returns active intervals in child/ID
// order, without the single-child query's pre-phase upcoming fallback.
func (m *Module) RequestChildOfferingsForChildrenAtDate(ctx context.Context, childIDs []int64, onDate Date) (rows []*RequestChildOffering, err error) {
	if len(childIDs) == 0 {
		return nil, nil
	}
	err = m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		rows, err = m.engine.RequestChildOfferingsForChildrenAtDate(ctx, childIDs, onDate)
		return err
	})
	return rows, err
}

// RequestChildOfferingHistoryForChildren returns all intervals in child/ID order.
func (m *Module) RequestChildOfferingHistoryForChildren(ctx context.Context, childIDs []int64) (rows []*RequestChildOffering, err error) {
	if len(childIDs) == 0 {
		return nil, nil
	}
	err = m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		rows, err = m.engine.RequestChildOfferingHistoryForChildren(ctx, childIDs)
		return err
	})
	return rows, err
}

// RequestChildOfferingsAtDates returns active intervals for each child's date,
// ordered by child and selection ID. It does not apply an upcoming fallback.
func (m *Module) RequestChildOfferingsAtDates(ctx context.Context, dates map[int64]Date) (rows []*RequestChildOffering, err error) {
	if len(dates) == 0 {
		return nil, nil
	}
	err = m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		rows, err = m.engine.RequestChildOfferingsAtDates(ctx, dates)
		return err
	})
	return rows, err
}

func (m *Module) ApprovedSelectionsForOfferings(ctx context.Context, offeringIDs []int64, onOrAfter Date) (rows []*ApprovedOfferingSelection, err error) {
	if len(offeringIDs) == 0 {
		return []*ApprovedOfferingSelection{}, nil
	}
	err = m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		rows, err = m.engine.ApprovedSelectionsForOfferings(ctx, offeringIDs, onOrAfter)
		return err
	})
	return rows, err
}

// ApprovedSelectionsForStudents returns intervals overlapping the inclusive
// date range. People Directory owns the exclusion of alumni.
func (m *Module) ApprovedSelectionsForStudents(ctx context.Context, ids []int64, from, to Date) (rows []*ApprovedOfferingSelection, err error) {
	if len(ids) == 0 || to.Before(from) {
		return []*ApprovedOfferingSelection{}, nil
	}
	err = m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		rows, err = m.engine.ApprovedSelectionsForStudents(ctx, ids, from, to)
		return err
	})
	return rows, err
}
