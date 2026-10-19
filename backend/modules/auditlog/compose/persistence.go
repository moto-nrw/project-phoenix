package compose

import (
	"errors"

	audit "github.com/moto-nrw/project-phoenix/modules/auditlog/internal/adapters/auditpostgres"
	"github.com/uptrace/bun"
)

// The ports below are what the booking consistency audit needs from other
// owners. The Audit composition declares them, so the consumers that bind
// People Directory, Care Plan and Enrollment implement them without the Audit
// Platform importing those owners (ADR 0049).
type (
	// DirectoryStudent is the People Directory row the alumnus exclusion reads.
	DirectoryStudent = audit.DirectoryStudent
	// StudentDirectory names the alumni among approved students.
	StudentDirectory = audit.StudentDirectory
	// CareOffering is the narrow Care Plan offering the audit checks against.
	CareOffering = audit.CareOfferingProjection
	// CareOfferingDirectory supplies the current tenant's care offerings.
	CareOfferingDirectory = audit.CareOfferingDirectory
	// EnrollmentBookings supplies approved bookings and their offering links.
	EnrollmentBookings = audit.EnrollmentQueries
	// Runtime resolves the caller's transaction and tenant.
	Runtime = audit.Runtime
	// Persistence is the Audit persistence the composition builds: the
	// append store, every retained repository, the booking consistency
	// audit and the operator audit ledger, each model-typed.
	Persistence = audit.Repositories
)

// NewFixedRuntime resolves the ambient transaction and validates the supplied
// school selector. The database is not a fallback connection.
var NewFixedRuntime = audit.NewRuntime

// PersistenceDependencies configure the Audit persistence a composition
// builds.
type PersistenceDependencies struct {
	// DB backs the ambient runtime used when Reads or Appends is nil.
	DB *bun.DB
	// Reads binds the retained repositories; nil reads through the Audit
	// handoff on the context, rejecting a missing transaction.
	Reads Runtime
	// Appends binds the append store; nil appends through Reads.
	Appends Runtime
	// Enrollment enables the booking consistency audit; without it the
	// bundle carries no BookingConsistency.
	Enrollment EnrollmentBookings
	// Students and CarePlan are bound into the booking consistency audit.
	Students StudentDirectory
	CarePlan CareOfferingDirectory
}

// NewPersistence builds the Audit runtime, every retained repository, the
// booking consistency audit and the operator audit ledger.
func NewPersistence(dependencies PersistenceDependencies) (*Persistence, error) {
	reads := dependencies.Reads
	if reads == nil {
		if dependencies.DB == nil {
			return nil, errors.New("audit persistence: database or read runtime is required")
		}
		reads = audit.NewAmbientRuntime(dependencies.DB)
	}
	appends := dependencies.Appends
	if appends == nil {
		appends = reads
	}
	repositories := audit.NewRepositories(reads, appends)
	if dependencies.Enrollment != nil {
		repositories.BookingConsistency = audit.NewBookingConsistencyRepository(reads, dependencies.Enrollment, dependencies.Students, dependencies.CarePlan)
	}
	return repositories, nil
}

// MustNewPersistence is NewPersistence for composition seams that cannot
// return an error; a missing database is a wiring error.
func MustNewPersistence(dependencies PersistenceDependencies) *Persistence {
	repositories, err := NewPersistence(dependencies)
	if err != nil {
		panic(err)
	}
	return repositories
}
