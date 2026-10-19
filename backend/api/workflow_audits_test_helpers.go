//go:build testhooks

package api

import (
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
)

// MustNewWorkflowAuditCommand is newWorkflowAuditCommand for test
// compositions that cannot return an error.
func MustNewWorkflowAuditCommand(db transactionruntime.Database, message string) auditModels.Command {
	command, err := owners.NewWorkflowAuditCommand(db, nil, message)
	if err != nil {
		panic(err)
	}
	return command
}
