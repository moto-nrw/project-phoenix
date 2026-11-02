package owners

import (
	"context"
	"fmt"

	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// payrollRuleAudit binds Workforce's payroll rule audit (#4807) to the
// settings audit: a change of a wage type, rule or mapping is one row in
// config.setting_audit, written in the request's tenant transaction.
type payrollRuleAudit struct {
	audit configModels.SettingAuditRepository
}

// NewPayrollRuleAudit writes over the settings runtime of the caller's
// transaction.
func NewPayrollRuleAudit(db transactionruntime.Database) workforceModule.PayrollRuleAudit {
	return payrollRuleAudit{audit: settingsCompose.NewSettingAuditRepository(settingsCompose.NewTenantSettingsRuntime(db.Pool(), nil))}
}

func (a payrollRuleAudit) RecordPayrollRuleChange(ctx context.Context, change workforceModule.PayrollRuleChange) error {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return fmt.Errorf("payroll rule audit: %w", err)
	}
	var changedBy *int64
	if change.ActorAccountID > 0 {
		actor := change.ActorAccountID
		changedBy = &actor
	}
	return a.audit.Create(ctx, configModels.NewAuditEntry(tenantID.Int64(), change.Key, change.Action, change.Old, change.New, changedBy))
}

// BuildPayrollRules composes the wage type and surcharge rule maintenance of
// /payroll over the Workforce rows and the settings audit.
func BuildPayrollRules(workTime workforceModule.Capability, db transactionruntime.Database) (workforceModule.PayrollRules, error) {
	return workTime.PayrollRuleMaintenance(NewPayrollRuleAudit(db))
}
