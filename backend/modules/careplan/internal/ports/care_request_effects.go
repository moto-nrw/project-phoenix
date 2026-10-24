package ports

import (
	"context"

	careplanNative "github.com/moto-nrw/project-phoenix/modules/careplan"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
)

// CareRequestRows reads the care request a decision reloads.
type CareRequestRows interface {
	FindCareScheduleRequest(ctx context.Context, id int64, lock bool) (carerequests.Request, error)
}

// CareRequestStudentNames names the child of a care request. The name only
// labels the review item; a failed lookup leaves it blank.
type CareRequestStudentNames interface {
	StudentName(ctx context.Context, studentID int64) (careplanNative.PersonName, error)
}

// CareScheduleBroadcaster wakes staff tabs after an approved care request
// changed the child's plan. Every call runs after commit.
type CareScheduleBroadcaster interface {
	// StudentUpdated invalidates the student master record of the school.
	StudentUpdated(tenantID int64) error
	// StudentCompanionsChanged refreshes the "läuft mit" links of the school.
	StudentCompanionsChanged(tenantID int64) error
	// ArrivalScheduleChanged refreshes arrival and pickup plans everywhere.
	ArrivalScheduleChanged() error
}

// GuardianAccess answers whether the submitting guardian still holds the
// child with parent portal access.
type GuardianAccess interface {
	GuardianHasChildAccess(ctx context.Context, studentID, guardianAccountID int64) (bool, error)
}
