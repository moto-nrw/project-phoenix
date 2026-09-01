package repositories

import (
	"context"

	audit "github.com/moto-nrw/project-phoenix/models/audit"
)

// RouteAuditWrites preserves the table-specific query interfaces while making
// the application command the sole write entry point.
func (f *Factory) RouteAuditWrites(command audit.Command) {
	if command == nil {
		panic("audit command is required")
	}
	f.DataDeletion = dataDeletionCommand{f.DataDeletion, command}
	f.StudentDeletionAudit = studentDeletionCommand{f.StudentDeletionAudit, command}
	f.EnrollmentDeletionAudit = enrollmentDeletionCommand{command}
	f.EnrollmentRestorationAudit = enrollmentRestorationCommand{command}
	f.DataAccessLog = dataAccessLogCommand{f.DataAccessLog, command}
	f.EnrollmentOfferingAdjustment = enrollmentOfferingAdjustmentCommand{f.EnrollmentOfferingAdjustment, command}
	f.GuardianChange = guardianChangeCommand{f.GuardianChange, command}
	f.DeviationEvent = deviationEventCommand{f.DeviationEvent, command}
	f.AuthEvent = authEventCommand{f.AuthEvent, command}
	f.DataImport = dataImportCommand{f.DataImport, command}
	f.WorkSessionEdit = workSessionEditCommand{f.WorkSessionEdit, command}
	f.StudentFieldEdit = studentFieldEditCommand{f.StudentFieldEdit, command}
	f.UnregisteredTagScan = unregisteredTagScanCommand{f.UnregisteredTagScan, command}
	f.TimeTrackingDeletion = timeTrackingDeletionCommand{command}
	f.PersonnelNumberChange = personnelNumberChangeCommand{command}
	f.StaffMasterDataChange = staffMasterDataChangeCommand{command}
	f.GuardianFinancialChange = guardianFinancialChangeCommand{command}
	f.ClassListEntryChange = classListEntryChangeCommand{f.ClassListEntryChange, command}
	f.FileEvent = fileEventCommand{f.FileEvent, command}
	f.SubstitutionChange = substitutionChangeCommand{command}
}

// RouteAuthEventWrites and RouteDataDeletionWrites keep standalone command
// roots on the same Audit command as the full service graph.
func RouteAuthEventWrites(repo audit.AuthEventRepository, command audit.Command) audit.AuthEventRepository {
	if repo == nil || command == nil {
		panic("auth event repository and audit command are required")
	}
	return authEventCommand{repo, command}
}

func RouteDataDeletionWrites(repo audit.DataDeletionRepository, command audit.Command) audit.DataDeletionRepository {
	if repo == nil || command == nil {
		panic("data deletion repository and audit command are required")
	}
	return dataDeletionCommand{repo, command}
}

type dataDeletionCommand struct {
	audit.DataDeletionRepository
	command audit.Command
}

func (r dataDeletionCommand) Create(ctx context.Context, event *audit.DataDeletion) error {
	return r.command.Append(ctx, event)
}

type studentDeletionCommand struct {
	audit.StudentDeletionRepository
	command audit.Command
}

func (r studentDeletionCommand) Create(ctx context.Context, event *audit.StudentDeletion) error {
	return r.command.Append(ctx, event)
}

type enrollmentDeletionCommand struct{ command audit.Command }

func (r enrollmentDeletionCommand) Create(ctx context.Context, event *audit.EnrollmentDeletion) error {
	return r.command.Append(ctx, event)
}

type enrollmentRestorationCommand struct{ command audit.Command }

func (r enrollmentRestorationCommand) Create(ctx context.Context, event *audit.EnrollmentRestoration) error {
	return r.command.Append(ctx, event)
}

type dataAccessLogCommand struct {
	audit.DataAccessLogRepository
	command audit.Command
}

func (r dataAccessLogCommand) Create(ctx context.Context, event *audit.DataAccessLog) error {
	return r.command.Append(ctx, event)
}

type enrollmentOfferingAdjustmentCommand struct {
	audit.EnrollmentOfferingAdjustmentRepository
	command audit.Command
}

func (r enrollmentOfferingAdjustmentCommand) Create(ctx context.Context, event *audit.EnrollmentOfferingAdjustment) error {
	return r.command.Append(ctx, event)
}

type guardianChangeCommand struct {
	audit.GuardianChangeRepository
	command audit.Command
}

func (r guardianChangeCommand) Create(ctx context.Context, event *audit.GuardianChange) error {
	return r.command.Append(ctx, event)
}

type deviationEventCommand struct {
	audit.DeviationEventRepository
	command audit.Command
}

func (r deviationEventCommand) Create(ctx context.Context, event *audit.DeviationEvent) error {
	return r.command.Append(ctx, event)
}

type authEventCommand struct {
	audit.AuthEventRepository
	command audit.Command
}

func (r authEventCommand) Create(ctx context.Context, event *audit.AuthEvent) error {
	return r.command.Append(ctx, event)
}

type dataImportCommand struct {
	audit.DataImportRepository
	command audit.Command
}

func (r dataImportCommand) Create(ctx context.Context, event *audit.DataImport) error {
	return r.command.Append(ctx, event)
}

type workSessionEditCommand struct {
	audit.WorkSessionEditRepository
	command audit.Command
}

func (r workSessionEditCommand) CreateBatch(ctx context.Context, events []*audit.WorkSessionEdit) error {
	for _, event := range events {
		if err := r.command.Append(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

type studentFieldEditCommand struct {
	audit.StudentFieldEditRepository
	command audit.Command
}

func (r studentFieldEditCommand) CreateBatch(ctx context.Context, events []*audit.StudentFieldEdit) error {
	for _, event := range events {
		if err := r.command.Append(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

type unregisteredTagScanCommand struct {
	audit.UnregisteredTagScanRepository
	command audit.Command
}

func (r unregisteredTagScanCommand) Create(ctx context.Context, event *audit.UnregisteredTagScan) error {
	return r.command.Append(ctx, event)
}

type timeTrackingDeletionCommand struct{ command audit.Command }

func (r timeTrackingDeletionCommand) Create(ctx context.Context, event *audit.TimeTrackingDeletion) error {
	return r.command.Append(ctx, event)
}

type personnelNumberChangeCommand struct{ command audit.Command }

func (r personnelNumberChangeCommand) Create(ctx context.Context, event *audit.PersonnelNumberChange) error {
	return r.command.Append(ctx, event)
}

type staffMasterDataChangeCommand struct{ command audit.Command }

func (r staffMasterDataChangeCommand) Create(ctx context.Context, event *audit.StaffMasterDataChange) error {
	return r.command.Append(ctx, event)
}

type guardianFinancialChangeCommand struct{ command audit.Command }

func (r guardianFinancialChangeCommand) Create(ctx context.Context, event *audit.GuardianFinancialChange) error {
	return r.command.Append(ctx, event)
}

type classListEntryChangeCommand struct {
	audit.ClassListEntryChangeRepository
	command audit.Command
}

func (r classListEntryChangeCommand) Create(ctx context.Context, event *audit.ClassListEntryChange) error {
	return r.command.Append(ctx, event)
}

type fileEventCommand struct {
	audit.FileEventRepository
	command audit.Command
}

func (r fileEventCommand) Create(ctx context.Context, event *audit.FileEvent) error {
	return r.command.Append(ctx, event)
}

type substitutionChangeCommand struct{ command audit.Command }

func (r substitutionChangeCommand) Create(ctx context.Context, event *audit.SubstitutionChange) error {
	return r.command.Append(ctx, event)
}
