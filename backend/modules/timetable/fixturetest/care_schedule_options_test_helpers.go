//go:build testhooks

package fixturetest

import (
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

func careScheduleListFilter(options *QueryOptions) (ports.CareScheduleFilter, error) {
	return narrowListOptions(options, "care schedule",
		func(f *ports.CareScheduleFilter, limit, offset int) { f.Limit, f.Offset = limit, offset },
		func(f *ports.CareScheduleFilter, field ListSortField) {
			f.Sort = append(f.Sort, ports.CareScheduleSort{Field: field.Field, Descending: field.Direction == ListSortDesc})
		},
		func(f *ports.CareScheduleFilter, condition ListCondition) error {
			id, ok := condition.Value.(int64)
			if condition.Field != "student_id" || condition.Operator != ListOpEqual || !ok || f.StudentIDs != nil {
				return fmt.Errorf("unsupported care schedule filter %s %s", condition.Field, condition.Operator)
			}
			f.StudentIDs = []int64{id}
			return nil
		})
}
