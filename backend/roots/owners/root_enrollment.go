package owners

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// EnrollmentRows are Enrollment as the roots bind it to the owners they
// composed (#4012): the booking projection over Care Plan's bookings, the
// approved bookings resolved to the People Directory's students, the
// booking writes into Care Plan, the guardian read of the deletion preview
// and the parents portal's reads of a child's care periods and bookings.
type EnrollmentRows struct {
	Bookings              *enrollmentOwner.BookingProjection
	ApprovedOfferings     *enrollmentCompose.ApprovedOfferingProjection
	CareBookings          EnrollmentCareBookings
	Guardians             enrollmentCompose.GuardianDirectory
	ParentCarePeriods     EnrollmentCarePeriods
	ParentOfferingHistory EnrollmentOfferingHistory
}

// buildEnrollmentRows binds Enrollment's booking projection to Care Plan's
// bookings and the People Directory.
func BuildEnrollmentRows(bookings *enrollmentOwner.BookingProjection, carePlan careplan.Capability, persons peopledirectory.Capability) EnrollmentRows {
	return EnrollmentRows{
		Bookings:              bookings,
		ApprovedOfferings:     enrollmentCompose.NewApprovedOfferingProjection(bookings, OfferingStudents{Query: persons}),
		CareBookings:          EnrollmentCareBookingsBinding{Owner: carePlan},
		Guardians:             enrollmentGuardianDirectory{guardians: persons},
		ParentCarePeriods:     parentCarePeriods{owner: bookings},
		ParentOfferingHistory: parentOfferingHistory{owner: bookings},
	}
}
