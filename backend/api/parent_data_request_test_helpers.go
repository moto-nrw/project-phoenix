//go:build testhooks

package api

import (
	"context"
	"errors"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// CreateParentDataRequestFixture persists the same complete native snapshot
// through the existing owner command and retains returned IDs and timestamps.
func CreateParentDataRequestFixture(source careplan.CareRequestsCommand, ctx context.Context, value *parentportal.StudentDataChangeRequest) error {
	if value == nil {
		return errors.New("StudentDataChangeRequest cannot be nil or zero value")
	}
	created, err := owners.NewParentDataRequestCommands(source).CreateStudentDataRequest(ctx, *value)
	if err == nil {
		*value = created
	}
	return err
}
