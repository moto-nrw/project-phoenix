package timetracking

import (
	"context"
	"testing"

	exportTransferModule "github.com/moto-nrw/project-phoenix/modules/exporttransfer"
)

// A stub Export Transfer capability for the workforce route tests (#3050).
//
// No school in these tests has a counterpart configured, which is the honest
// default: every transfer answers "not configured", and the two SFTP routes
// still exist so the route and middleware goldens cover them.
//
// A stub rather than the real composition on purpose: wiring the module here
// would drag the composition packages into the workforce HTTP tests, and what
// these tests check is the route surface, not the transfer.
type stubExportTransfer struct{}

func (stubExportTransfer) Status(context.Context) (exportTransferModule.Status, error) {
	return exportTransferModule.Status{}, nil
}

func (stubExportTransfer) Transfer(_ context.Context, request exportTransferModule.Request) (exportTransferModule.Outcome, error) {
	return exportTransferModule.Outcome{
		Filename: request.Filename,
		Reason:   exportTransferModule.ReasonNotConfigured,
	}, nil
}

func testExportTransferModule(t *testing.T) *exportTransferModule.Module {
	t.Helper()
	return exportTransferModule.NewModule(stubExportTransfer{})
}
