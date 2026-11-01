package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application"
	"github.com/uptrace/bun"
)

// NewPayrollRules builds the maintenance of the school's own wage types and
// surcharge rules (#4807); the root binds the settings audit.
func NewPayrollRules(db *bun.DB, audit workforce.PayrollRuleAudit) (workforce.PayrollRules, error) {
	if db == nil {
		return nil, errors.New("payroll rules: database is required")
	}
	return application.NewPayrollRules(postgres.New(postgres.DatabaseRuntime(db)), audit)
}

// NewPayrollRuleSetReader builds the reader the export and the month close
// take the rule set in force from (#4807).
func NewPayrollRuleSetReader(db *bun.DB) (workforce.PayrollRuleSetReader, error) {
	if db == nil {
		return nil, errors.New("payroll rule set reader: database is required")
	}
	return application.NewPayrollRuleSetReader(postgres.New(postgres.DatabaseRuntime(db))), nil
}
