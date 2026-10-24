package timetracking

import (
	"context"
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

func (stubExportTransfer) Status(context.Context) (ExportTransferStatus, error) {
	return ExportTransferStatus{}, nil
}

func (stubExportTransfer) Transfer(_ context.Context, request ExportTransferRequest) (ExportTransferOutcome, error) {
	return ExportTransferOutcome{
		Filename: request.Filename,
		Reason:   exportTransferReasonNotConfigured,
	}, nil
}
