package owners

import (
	"errors"

	projection "github.com/moto-ogs/project-phoenix/modules/timetableprojection/compose"
	"github.com/moto-ogs/project-phoenix/tenant"
	"github.com/moto-ogs/project-phoenix/workflows/studentdeletion"
)

func NewStudentActivityEnrollments() studentdeletion.ActivityEnrollments {
	return projection.New().StudentEnrollmentsFromContext(tenant.TransactionFromContext, tenant.FromContext, errors.New("student deletion: activity enrollment count requires a tenant transaction"))
}
