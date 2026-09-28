package testutil

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	"github.com/moto-nrw/project-phoenix/services"
)

// The enrollment flow suites (#3565) bind their doubles to these ports and
// drive these compositions over the test database without importing the
// composition root.
type (
	EnrollmentIntakeRequests        = services.EnrollmentIntakeRequests
	EnrollmentIntakeChildren        = services.EnrollmentIntakeChildren
	EnrollmentIntakeGuardians       = services.EnrollmentIntakeGuardians
	EnrollmentIntakeLateInvites     = services.EnrollmentIntakeLateInvites
	EnrollmentIntakeCatalog         = services.EnrollmentIntakeCatalog
	EnrollmentSubmissionRateLimiter = services.EnrollmentSubmissionRateLimiter
	EnrollmentGuardianAuthorizer    = services.EnrollmentGuardianAuthorizer
	EnrollmentChangeRequestRecords  = services.EnrollmentChangeRequestRecords

	EnrollmentFlowDataAccessLog           = services.EnrollmentFlowDataAccessLog
	EnrollmentFlowDataAccessLogs          = services.EnrollmentFlowDataAccessLogs
	EnrollmentFlowDeletionAudit           = services.EnrollmentFlowDeletionAudit
	EnrollmentFlowDeletionAudits          = services.EnrollmentFlowDeletionAudits
	EnrollmentFlowOfferingAdjustmentAudit = services.EnrollmentFlowOfferingAdjustmentAudit
	EnrollmentFlowRestorationAudit        = services.EnrollmentFlowRestorationAudit
)

// Resource types of the enrollment exports in the data-access log.
const (
	EnrollmentFlowPhaseExportResource   = services.EnrollmentFlowPhaseExportResource
	EnrollmentFlowStudentExportResource = services.EnrollmentFlowStudentExportResource
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
	// NewEnrollmentFlowDelivery composes the Delivery platform with a
	// provider that accepts every send.
	NewEnrollmentFlowDelivery = services.NewEnrollmentFlowDelivery
	// NewEnrollmentFlowStudentAudit records student changes with the
	// request's audit actor.
	NewEnrollmentFlowStudentAudit = services.NewEnrollmentFlowStudentAudit
	// NewEnrollmentFlowPickupExcusal composes Care Plan's pickup
	// auto-excusal the way the root binds it.
	NewEnrollmentFlowPickupExcusal = services.NewEnrollmentFlowPickupExcusal
)
