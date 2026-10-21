package compose

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// The template, materialization, series conversion, roster and conflict
// services live in internal/application (#4084). The roots build them through
// the constructors below; their dependency types are in
// modules/timetable/ports.

func NewConflictDetection(deps ports.ConflictDetectionDependencies) (timetable.ConflictDetectionCapability, error) {
	if deps.RunInTx == nil {
		deps.RunInTx = tenant.NewTransactionRunner().RunInTx
	}
	return application.NewConflictDetection(deps)
}

func NewInstanceSeriesConversion(deps ports.InstanceSeriesConversionDependencies) (timetable.InstanceSeriesConversion, error) {
	return application.NewInstanceSeriesConversion(deps)
}

func NewMaterialization(deps ports.MaterializationDependencies) (timetable.MaterializationCapability, error) {
	return application.NewMaterialization(deps)
}

func NewTemplateAdministration(deps ports.TemplateAdministrationDependencies) (timetable.TemplateAdministration, error) {
	return application.NewTemplateAdministration(deps)
}

func NewRosterReconciler(
	instanceRepo ports.InstanceRows,
	instanceStudentRepo ports.ParticipantRows,
	enrollments timetable.StudentEnrollmentQuery,
	directories ports.RepositoryDirectories,
	logger *slog.Logger,
	now ...func() time.Time,
) ports.RosterReconciler {
	return application.NewRosterReconciler(instanceRepo, instanceStudentRepo, enrollments, directories, logger, now...)
}
