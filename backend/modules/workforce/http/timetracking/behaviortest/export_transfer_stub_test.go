package behaviortest

import (
	"context"

	timetracking "github.com/moto-ogs/project-phoenix/modules/workforce/http/timetracking"
)

// A stub Export Transfer port for the workforce route tests (#3050).
//
// No school in these tests has a counterpart configured, which is the honest
// default: every transfer answers "not configured", and the two SFTP routes
// still exist so the route and middleware goldens cover them.
//
// A stub rather than the real composition on purpose: wiring the module here
// would drag the composition packages into the workforce HTTP tests, and what
// these tests check is the route surface, not the transfer.
type stubExportTransfer struct{}

func (stubExportTransfer) Status(context.Context) (timetracking.ExportTransferStatus, error) {
	return timetracking.ExportTransferStatus{}, nil
}

func (stubExportTransfer) Transfer(_ context.Context, request timetracking.ExportTransferRequest) (timetracking.ExportTransferOutcome, error) {
	return timetracking.ExportTransferOutcome{
		Filename: request.Filename,
		Reason:   timetracking.ExportTransferReasonNotConfiguredForTest,
	}, nil
}
