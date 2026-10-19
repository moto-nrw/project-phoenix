package owners

import (
	"context"
	"fmt"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// guardianPaymentFields maps the port's field values to the audit trail's
// field names.
var guardianPaymentFields = map[peoplePorts.GuardianPaymentField]string{
	peoplePorts.GuardianPaymentFieldIBAN:          auditModels.GuardianPaymentFieldIBAN,
	peoplePorts.GuardianPaymentFieldAccountHolder: auditModels.GuardianPaymentFieldAccountHolder,
	peoplePorts.GuardianPaymentFieldIsPayer:       auditModels.GuardianPaymentFieldIsPayer,
}

// guardianPaymentAccesses maps the port's read kinds to the data-access log's
// resource types.
var guardianPaymentAccesses = map[peoplePorts.GuardianPaymentAccess]string{
	peoplePorts.GuardianPaymentAccessView:     auditModels.ResourceTypeGuardianFinancialView,
	peoplePorts.GuardianPaymentAccessReveal:   auditModels.ResourceTypeGuardianFinancialReveal,
	peoplePorts.GuardianPaymentAccessOverview: auditModels.ResourceTypeGuardianPaymentOverview,
	peoplePorts.GuardianPaymentAccessExport:   auditModels.ResourceTypeGuardianPaymentExport,
}

// guardianFinancialAudit writes the guardian bank-data changes to
// audit.guardian_financial_changes.
type guardianFinancialAudit struct {
	changes auditModels.GuardianFinancialChangeCreator
}

// NewGuardianFinancialAudit binds the change-trail port. A nil repository
// leaves the port unbound, so the service refuses the change instead of
// committing it without an audit row.
func NewGuardianFinancialAudit(changes auditModels.GuardianFinancialChangeCreator) peoplePorts.GuardianFinancialAudit {
	if changes == nil {
		return nil
	}
	return guardianFinancialAudit{changes: changes}
}

func (a guardianFinancialAudit) RecordGuardianFinancialChange(ctx context.Context, change peoplePorts.GuardianFinancialChange) error {
	field, ok := guardianPaymentFields[change.Field]
	if !ok {
		return fmt.Errorf("unknown guardian payment field %d", change.Field)
	}
	return a.changes.Create(ctx, &auditModels.GuardianFinancialChange{
		GuardianProfileID: change.GuardianProfileID,
		StudentID:         change.StudentID,
		ChangedBy:         change.ChangedBy,
		FieldName:         field,
		OldValue:          change.OldValue,
		NewValue:          change.NewValue,
		Note:              change.Note,
	})
}

// guardianDataAccessLog writes the guardian bank-data reads to
// audit.data_access_logs.
type guardianDataAccessLog struct {
	log DataAccessAppender
}

// NewGuardianDataAccessLog binds the read-trail port. A nil repository leaves
// the port unbound, so the service serves no bank value.
func NewGuardianDataAccessLog(log DataAccessAppender) peoplePorts.GuardianDataAccessLog {
	if log == nil {
		return nil
	}
	return guardianDataAccessLog{log: log}
}

func (l guardianDataAccessLog) RecordGuardianDataAccess(ctx context.Context, access peoplePorts.GuardianDataAccess) error {
	resourceType, ok := guardianPaymentAccesses[access.Access]
	if !ok {
		return fmt.Errorf("unknown guardian payment access %d", access.Access)
	}
	return l.log.Create(ctx, &auditModels.DataAccessLog{
		ActorAccountID: access.ActorAccountID,
		ActorRole:      access.ActorRole,
		ResourceType:   resourceType,
		RangeStart:     access.AccessedAt,
		RangeEnd:       access.AccessedAt,
		AccessedAt:     access.AccessedAt,
		Metadata:       access.Metadata,
	})
}
