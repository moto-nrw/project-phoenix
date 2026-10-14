package testutil

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
)

var (
	// NewTestCarePlan composes Care Plan's capability over the test
	// database.
	NewTestCarePlan = careplantest.NewCarePlan
	// NewTestOfferingBookings composes Care Plan's effective-booking owner.
	NewTestOfferingBookings = careplantest.NewOfferingBookings
	// NewStoredPickupBaselines reads pickup baselines with booking
	// authority off.
	NewStoredPickupBaselines = careplantest.NewStoredPickupBaselines
)
