package owners

import (
	"context"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	delivery "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewHolidayRegistrationRetention binds mail erasure to the registration's
// existing administrative command. Failure rolls back both owners' changes.
func NewHolidayRegistrationRetention(today func() calendar.Date) enrollment.HolidayRegistrationRetention {
	return enrollmentCompose.NewHolidayRegistrationRetention(holidayRegistrationMailErasure{deliveryCompose.NewHolidayRegistrationMailErasure()}, today)
}

type holidayRegistrationMailErasure struct {
	delivery.HolidayRegistrationMailErasure
}

func (m holidayRegistrationMailErasure) EraseHolidayRegistrationMails(ctx context.Context, registrations []enrollment.HolidayRegistrationReference) (int, error) {
	organizations, ids := make([]int64, len(registrations)), make([]int64, len(registrations))
	for i, registration := range registrations {
		organizations[i], ids[i] = registration.OrganizationID, registration.RegistrationID
	}
	return m.HolidayRegistrationMailErasure.EraseHolidayRegistrationMails(ctx, organizations, ids)
}
