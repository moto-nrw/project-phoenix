package audit

import (
	"context"
	"fmt"
	"reflect"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/uptrace/bun"
)

// Appender is the only Audit persistence adapter allowed to insert ledger
// rows. Read repositories stay table-specific.
type Appender struct{ runtime Runtime }

func NewAppender(runtime Runtime) *Appender {
	return &Appender{runtime: requireRuntime(runtime)}
}

func (a *Appender) Append(ctx context.Context, event any) error {
	db, err := a.prepare(ctx, event)
	if err != nil {
		return err
	}
	table, err := appendEvent(ctx, db, event)
	return wrapDatabase("append "+table, err)
}

// AppendOnce appends an event subject to its database uniqueness constraint
// and reports whether this call inserted the row.
func (a *Appender) AppendOnce(ctx context.Context, event any) (bool, error) {
	db, err := a.prepare(ctx, event)
	if err != nil {
		return false, err
	}
	table, inserted, err := appendEventOnce(ctx, db, event)
	return inserted, wrapDatabase("append once "+table, err)
}

func (a *Appender) prepare(ctx context.Context, event any) (bun.IDB, error) {
	if event == nil || (reflect.ValueOf(event).Kind() == reflect.Pointer && reflect.ValueOf(event).IsNil()) {
		return nil, fmt.Errorf("audit event is required")
	}
	scoped, ok := event.(interface {
		GetTenantID() int64
		SetTenantID(int64)
	})
	if !ok {
		return nil, fmt.Errorf("audit event %T is not tenant scoped", event)
	}
	if err := validateEvent(event); err != nil {
		return nil, err
	}
	return prepareTenant(ctx, a.runtime, scoped)
}

func appendEvent(ctx context.Context, db bun.IDB, event any) (string, error) {
	table, query, err := appendQuery(db, event)
	if err != nil {
		return table, err
	}
	_, err = query.Exec(ctx)
	return table, err
}

func appendEventOnce(ctx context.Context, db bun.IDB, event any) (string, bool, error) {
	table, query, err := appendQuery(db, event)
	if err != nil {
		return table, false, err
	}
	result, err := query.On("CONFLICT DO NOTHING").Exec(ctx)
	if err != nil {
		return table, false, err
	}
	rows, err := result.RowsAffected()
	return table, rows > 0, err
}

func appendQuery(db bun.IDB, event any) (string, *bun.InsertQuery, error) {
	var query *bun.InsertQuery
	var table string
	switch value := event.(type) {
	case *auditModels.AttendanceCorrection:
		table, query = "audit.attendance_corrections", db.NewInsert().Model(value).ModelTableExpr("audit.attendance_corrections")
	case *auditModels.AuthEvent:
		table, query = "audit.auth_events", db.NewInsert().Model(value).ModelTableExpr("audit.auth_events")
	case *auditModels.ClassListEntryChange:
		table, query = "audit.class_list_entry_changes", db.NewInsert().Model(value).ModelTableExpr("audit.class_list_entry_changes")
	case *auditModels.DataAccessLog:
		table, query = "audit.data_access_log", db.NewInsert().Model(value).ModelTableExpr("audit.data_access_log")
	case *auditModels.DataDeletion:
		table, query = "audit.data_deletions", db.NewInsert().Model(value).ModelTableExpr("audit.data_deletions")
	case *auditModels.DataImport:
		table, query = "audit.data_imports", db.NewInsert().Model(value).ModelTableExpr("audit.data_imports")
	case *auditModels.DeviationEvent:
		table, query = "audit.deviation_events", db.NewInsert().Model(value).ModelTableExpr("audit.deviation_events")
	case *auditModels.EnrollmentDeletion:
		table, query = "audit.enrollment_deletions", db.NewInsert().Model(value).ModelTableExpr("audit.enrollment_deletions")
	case *auditModels.EnrollmentOfferingAdjustment:
		table, query = "audit.enrollment_offering_adjustments", db.NewInsert().Model(value).ModelTableExpr("audit.enrollment_offering_adjustments")
	case *auditModels.EnrollmentRestoration:
		table, query = "audit.enrollment_restorations", db.NewInsert().Model(value).ModelTableExpr("audit.enrollment_restorations")
	case *auditModels.FileEvent:
		table, query = "audit.file_events", db.NewInsert().Model(value).ModelTableExpr("audit.file_event_ledger")
	case *auditModels.GuardianChange:
		table, query = "audit.guardian_changes", db.NewInsert().Model(value).ModelTableExpr("audit.guardian_changes")
	case *auditModels.GuardianFinancialChange:
		table, query = "audit.guardian_financial_changes", db.NewInsert().Model(value).ModelTableExpr("audit.guardian_financial_change_ledger")
	case *auditModels.PersonnelNumberChange:
		table, query = "audit.personnel_number_changes", db.NewInsert().Model(value).ModelTableExpr("audit.personnel_number_changes")
	case *auditModels.StaffMasterDataChange:
		table, query = "audit.staff_master_data_changes", db.NewInsert().Model(value).ModelTableExpr("audit.staff_master_data_changes")
	case *auditModels.StudentDeletion:
		table, query = "audit.student_deletions", db.NewInsert().Model(value).ModelTableExpr("audit.student_deletions")
	case *auditModels.StudentFieldEdit:
		table, query = "audit.student_field_edits", db.NewInsert().Model(value).ModelTableExpr("audit.student_field_edits")
	case *auditModels.StudentConsentChange:
		table, query = "audit.student_consent_changes", db.NewInsert().Model(value).ModelTableExpr("audit.student_consent_changes")
	case *auditModels.SubstitutionChange:
		table, query = "audit.substitution_changes", db.NewInsert().Model(value).ModelTableExpr("audit.substitution_changes")
	case *auditModels.TimeTrackingDeletion:
		table, query = "audit.time_tracking_deletions", db.NewInsert().Model(value).ModelTableExpr("audit.time_tracking_deletions")
	case *auditModels.UnregisteredTagScan:
		table, query = "audit.unregistered_tag_scans", db.NewInsert().Model(value).ModelTableExpr("audit.unregistered_tag_scans")
	case *auditModels.WorkSessionEdit:
		table, query = "audit.work_session_edits", db.NewInsert().Model(value).ModelTableExpr("audit.work_session_edits")
	default:
		return "event", nil, fmt.Errorf("unsupported event type %T", event)
	}
	return table, query, nil
}

func validateEvent(event any) error {
	var err error
	switch value := event.(type) {
	case *auditModels.AttendanceCorrection:
		err = value.Validate()
	case *auditModels.AuthEvent:
		err = value.Validate()
	case *auditModels.ClassListEntryChange:
		err = value.Validate()
	case *auditModels.DataDeletion:
		err = value.Validate()
	case *auditModels.DeviationEvent:
		err = value.Validate()
	case *auditModels.FileEvent:
		err = value.Validate()
	case *auditModels.GuardianFinancialChange:
		err = value.Validate()
	case *auditModels.PersonnelNumberChange:
		err = value.Validate()
	case *auditModels.StaffMasterDataChange:
		err = value.Validate()
	case *auditModels.StudentFieldEdit:
		err = value.Validate()
	case *auditModels.TimeTrackingDeletion:
		err = value.Validate()
	case *auditModels.WorkSessionEdit:
		err = value.Validate()
	}
	if err != nil {
		return fmt.Errorf("invalid audit event %T: %w", event, err)
	}
	return nil
}
