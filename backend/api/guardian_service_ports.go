package api

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/services"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
)

// The guardian service's ports (#3755). It moved from services/users into
// People Directory's composition, which may name neither the audit trail
// nor the authorization policy; the root binds both here with the field
// names, resource types and role presets the service used before.

// guardianPaymentFields maps the port's field values to the audit trail's
// field names.
var guardianPaymentFields = map[peopleCompose.GuardianPaymentField]string{
	peopleCompose.GuardianPaymentFieldIBAN:          auditModels.GuardianPaymentFieldIBAN,
	peopleCompose.GuardianPaymentFieldAccountHolder: auditModels.GuardianPaymentFieldAccountHolder,
	peopleCompose.GuardianPaymentFieldIsPayer:       auditModels.GuardianPaymentFieldIsPayer,
}

// guardianPaymentAccesses maps the port's read kinds to the data-access log's
// resource types.
var guardianPaymentAccesses = map[peopleCompose.GuardianPaymentAccess]string{
	peopleCompose.GuardianPaymentAccessView:     auditModels.ResourceTypeGuardianFinancialView,
	peopleCompose.GuardianPaymentAccessReveal:   auditModels.ResourceTypeGuardianFinancialReveal,
	peopleCompose.GuardianPaymentAccessOverview: auditModels.ResourceTypeGuardianPaymentOverview,
	peopleCompose.GuardianPaymentAccessExport:   auditModels.ResourceTypeGuardianPaymentExport,
}

// guardianFinancialAudit writes the guardian bank-data changes to
// audit.guardian_financial_changes.
type guardianFinancialAudit struct {
	changes auditModels.GuardianFinancialChangeCreator
}

// NewGuardianFinancialAudit binds the change-trail port. A nil repository
// leaves the port unbound, so the service refuses the change instead of
// committing it without an audit row.
func NewGuardianFinancialAudit(changes auditModels.GuardianFinancialChangeCreator) peopleCompose.GuardianFinancialAudit {
	if changes == nil {
		return nil
	}
	return guardianFinancialAudit{changes: changes}
}

func (a guardianFinancialAudit) RecordGuardianFinancialChange(ctx context.Context, change peopleCompose.GuardianFinancialChange) error {
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
	log services.DataAccessAppender
}

// NewGuardianDataAccessLog binds the read-trail port. A nil repository leaves
// the port unbound, so the service serves no bank value.
func NewGuardianDataAccessLog(log services.DataAccessAppender) peopleCompose.GuardianDataAccessLog {
	if log == nil {
		return nil
	}
	return guardianDataAccessLog{log: log}
}

func (l guardianDataAccessLog) RecordGuardianDataAccess(ctx context.Context, access peopleCompose.GuardianDataAccess) error {
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
