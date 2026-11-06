package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/traegerview"
)

// KeyDateCounts returns the captured Stichtagszahlen of every school of the
// Träger and the sum of each month (#4673). The rows are those of the operator
// billing report, read at Organisation & Tenancy for the schools of the Träger
// only; the projection keeps only rows of the school set it resolved. They are
// aggregated numbers, so the read is not written to the access log (ADR 0084
// section 7).
func (o *Overview) KeyDateCounts(ctx context.Context, organizationID int64) (traegerview.KeyDateCounts, error) {
	if organizationID <= 0 {
		return traegerview.KeyDateCounts{}, traegerview.ErrTraegerRequired
	}
	var result traegerview.KeyDateCounts
	err := o.read(ctx, organizationID, func(adminCtx context.Context, ids []int64) error {
		if len(ids) == 0 {
			result = keyDateCounts(nil, nil)
			return nil
		}
		schools, err := o.rows.Schools(adminCtx, ids)
		if err != nil {
			return err
		}
		counts, err := o.keyDates.ListTraegerKeyDateCounts(adminCtx, organizationID)
		if err != nil {
			return err
		}
		result = keyDateCounts(schools, counts)
		return nil
	})
	if err != nil {
		return traegerview.KeyDateCounts{}, err
	}
	return result, nil
}

// keyDateCounts pairs each school with its months and sums each month over
// the schools. counts arrive newest month first; a row of a school outside
// schools is dropped, so the answer never carries a school the Träger does
// not have.
func keyDateCounts(schools []traegerview.School, counts []traegerview.SchoolKeyDateCount) traegerview.KeyDateCounts {
	result := traegerview.KeyDateCounts{
		Schools: make([]traegerview.SchoolKeyDateCounts, 0, len(schools)),
		Months:  []traegerview.KeyDateMonth{},
	}
	bySchool := make(map[int64][]traegerview.KeyDateCount, len(schools))
	known := make(map[int64]bool, len(schools))
	for _, school := range schools {
		known[school.ID] = true
	}
	monthIndex := map[string]int{}
	for _, count := range counts {
		if !known[count.SchoolID] {
			continue
		}
		bySchool[count.SchoolID] = append(bySchool[count.SchoolID], count.KeyDateCount)
		index, ok := monthIndex[count.Period.String()]
		if !ok {
			index = len(result.Months)
			monthIndex[count.Period.String()] = index
			result.Months = append(result.Months, traegerview.KeyDateMonth{Period: count.Period, KeyDate: count.KeyDate})
		}
		month := &result.Months[index]
		month.ActiveStudents += count.ActiveStudents
		month.ActiveTerminals += count.ActiveTerminals
		month.Schools++
	}
	for _, school := range schools {
		months := bySchool[school.ID]
		if months == nil {
			months = []traegerview.KeyDateCount{}
		}
		result.Schools = append(result.Schools, traegerview.SchoolKeyDateCounts{School: school, Counts: months})
	}
	return result
}
