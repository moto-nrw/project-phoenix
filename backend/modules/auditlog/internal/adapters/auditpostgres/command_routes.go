package audit

import (
	"context"
	"fmt"

	auditModel "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
)

// WithRoutedWrites returns the repositories with every write routed through
// command. It preserves the table-specific query interfaces while making the
// application command the sole write entry point; the receiver is unchanged.
func (r *Repositories) WithRoutedWrites(command auditModel.Command) *Repositories {
	if command == nil {
		panic("audit command is required")
	}
	return &Repositories{
		AppendStore:                  r.AppendStore,
		FileEvent:                    fileEventCommand{r.FileEvent, command},
		SubstitutionChange:           substitutionChangeCommand{command},
		DataDeletion:                 dataDeletionCommand{r.DataDeletion, command},
		StudentDeletionAudit:         studentDeletionCommand{r.StudentDeletionAudit, command},
		EnrollmentDeletionAudit:      enrollmentDeletionCommand{command},
		EnrollmentRestorationAudit:   enrollmentRestorationCommand{command},
		DataAccessLog:                dataAccessLogCommand{r.DataAccessLog, command},
		EnrollmentOfferingAdjustment: enrollmentOfferingAdjustmentCommand{r.EnrollmentOfferingAdjustment, command},
		GuardianChange:               guardianChangeCommand{r.GuardianChange, command},
		StudentConsentChange:         studentConsentChangeCommand{r.StudentConsentChange, command},
		DeviationEvent:               deviationEventCommand{r.DeviationEvent, command},
		AuthEvent:                    authEventCommand{r.AuthEvent, command},
		DataImport:                   dataImportCommand{r.DataImport, command},
		WorkSessionEdit:              workSessionEditCommand{r.WorkSessionEdit, command},
		StudentFieldEdit:             studentFieldEditCommand{r.StudentFieldEdit, command},
		TimeTrackingDeletion:         timeTrackingDeletionCommand{command},
		PersonnelNumberChange:        personnelNumberChangeCommand{command},
		StaffMasterDataChange:        staffMasterDataChangeCommand{command},
		GuardianFinancialChange:      guardianFinancialChangeCommand{command},
		ClassListEntryChange:         classListEntryChangeCommand{r.ClassListEntryChange, command},
		TimeTrackingAuditLog:         r.TimeTrackingAuditLog,
		AttendanceCorrection:         r.AttendanceCorrection,
		OperatorAuditLog:             r.OperatorAuditLog,
		BookingConsistency:           r.BookingConsistency,
		RetentionSummaries:           r.RetentionSummaries,
	}
}

type dataDeletionCommand struct {
	auditModel.DataDeletionRepository
	command auditModel.Command
}

func (r dataDeletionCommand) Create(ctx context.Context, event *auditModel.DataDeletion) error {
	return r.command.Append(ctx, event)
}

type studentDeletionCommand struct {
	auditModel.StudentDeletionRepository
	command auditModel.Command
}

func (r studentDeletionCommand) Create(ctx context.Context, event *auditModel.StudentDeletion) error {
	return r.command.Append(ctx, event)
}

type enrollmentDeletionCommand struct{ command auditModel.Command }

func (r enrollmentDeletionCommand) Create(ctx context.Context, event *auditModel.EnrollmentDeletion) error {
	return r.command.Append(ctx, event)
}

type enrollmentRestorationCommand struct{ command auditModel.Command }

func (r enrollmentRestorationCommand) Create(ctx context.Context, event *auditModel.EnrollmentRestoration) error {
	return r.command.Append(ctx, event)
}

type dataAccessLogCommand struct {
	auditModel.DataAccessLogRepository
	command auditModel.Command
}

func (r dataAccessLogCommand) Create(ctx context.Context, event *auditModel.DataAccessLog) error {
	return r.command.Append(ctx, event)
}

type enrollmentOfferingAdjustmentCommand struct {
	auditModel.EnrollmentOfferingAdjustmentRepository
	command auditModel.Command
}

func (r enrollmentOfferingAdjustmentCommand) Create(ctx context.Context, event *auditModel.EnrollmentOfferingAdjustment) error {
	return r.command.Append(ctx, event)
}

type guardianChangeCommand struct {
	auditModel.GuardianChangeRepository
	command auditModel.Command
}

func (r guardianChangeCommand) Create(ctx context.Context, event *auditModel.GuardianChange) error {
	return r.command.Append(ctx, event)
}

type studentConsentChangeCommand struct {
	auditModel.StudentConsentChangeRepository
	command auditModel.Command
}

func (r studentConsentChangeCommand) Create(ctx context.Context, event *auditModel.StudentConsentChange) error {
	return r.command.Append(ctx, event)
}

type deviationEventCommand struct {
	auditModel.DeviationEventRepository
	command auditModel.Command
}

func (r deviationEventCommand) Create(ctx context.Context, event *auditModel.DeviationEvent) error {
	return r.command.Append(ctx, event)
}

type authEventCommand struct {
	auditModel.AuthEventRepository
	command auditModel.Command
}

func (r authEventCommand) Create(ctx context.Context, event *auditModel.AuthEvent) error {
	return r.command.Append(ctx, event)
}

func (r authEventCommand) CreateStaffPreviewEndOnce(ctx context.Context, event *auditModel.AuthEvent) (bool, error) {
	if event == nil || event.EventType != auditModel.EventTypeStaffPreviewEnded {
		return false, fmt.Errorf("staff preview end event is required")
	}
	command, ok := r.command.(auditModel.AppendOnceCommand)
	if !ok {
		return false, fmt.Errorf("audit command does not support append once")
	}
	return command.AppendOnce(ctx, event)
}

type dataImportCommand struct {
	auditModel.DataImportRepository
	command auditModel.Command
}

func (r dataImportCommand) Create(ctx context.Context, event *auditModel.DataImport) error {
	return r.command.Append(ctx, event)
}

type workSessionEditCommand struct {
	auditModel.WorkSessionEditRepository
	command auditModel.Command
}

func (r workSessionEditCommand) CreateBatch(ctx context.Context, events []*auditModel.WorkSessionEdit) error {
	for _, event := range events {
		if err := r.command.Append(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

type studentFieldEditCommand struct {
	auditModel.StudentFieldEditRepository
	command auditModel.Command
}

func (r studentFieldEditCommand) CreateBatch(ctx context.Context, events []*auditModel.StudentFieldEdit) error {
	for _, event := range events {
		if err := r.command.Append(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

type timeTrackingDeletionCommand struct{ command auditModel.Command }

func (r timeTrackingDeletionCommand) Create(ctx context.Context, event *auditModel.TimeTrackingDeletion) error {
	return r.command.Append(ctx, event)
}

type personnelNumberChangeCommand struct{ command auditModel.Command }

func (r personnelNumberChangeCommand) Create(ctx context.Context, event *auditModel.PersonnelNumberChange) error {
	return r.command.Append(ctx, event)
}

type staffMasterDataChangeCommand struct{ command auditModel.Command }

func (r staffMasterDataChangeCommand) Create(ctx context.Context, event *auditModel.StaffMasterDataChange) error {
	return r.command.Append(ctx, event)
}

type guardianFinancialChangeCommand struct{ command auditModel.Command }

func (r guardianFinancialChangeCommand) Create(ctx context.Context, event *auditModel.GuardianFinancialChange) error {
	return r.command.Append(ctx, event)
}

type classListEntryChangeCommand struct {
	auditModel.ClassListEntryChangeRepository
	command auditModel.Command
}

func (r classListEntryChangeCommand) Create(ctx context.Context, event *auditModel.ClassListEntryChange) error {
	return r.command.Append(ctx, event)
}

type fileEventCommand struct {
	auditModel.FileEventRepository
	command auditModel.Command
}

func (r fileEventCommand) Create(ctx context.Context, event *auditModel.FileEvent) error {
	return r.command.Append(ctx, event)
}

type substitutionChangeCommand struct{ command auditModel.Command }

func (r substitutionChangeCommand) Create(ctx context.Context, event *auditModel.SubstitutionChange) error {
	return r.command.Append(ctx, event)
}
