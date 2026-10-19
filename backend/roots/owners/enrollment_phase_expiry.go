package owners

import (
	"context"

	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

type OfferingStudents struct{ Query peopledirectory.StudentQuery }

func (d OfferingStudents) ListOfferingStudents(ctx context.Context, ids []int64) ([]enrollmentCompose.OfferingStudent, error) {
	values, err := d.Query.ListStudentsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	rows := make([]enrollmentCompose.OfferingStudent, 0, len(values))
	for _, value := range values {
		rows = append(rows, enrollmentCompose.OfferingStudent{ID: value.ID, SchoolClass: value.SchoolClass, Alumnus: value.IsAlumnus()})
	}
	return rows, nil
}
