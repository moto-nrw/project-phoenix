package timetracking

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
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

type preparedExportTransfer struct{ requests chan ExportTransferRequest }

func (p *preparedExportTransfer) Status(context.Context) (ExportTransferStatus, error) {
	return ExportTransferStatus{Ready: true}, nil
}

func (p *preparedExportTransfer) Transfer(_ context.Context, request ExportTransferRequest) (ExportTransferOutcome, error) {
	p.requests <- request
	return ExportTransferOutcome{}, nil
}

type callbackExportTransfer struct{}

func (callbackExportTransfer) Status(context.Context) (ExportTransferStatus, error) {
	return ExportTransferStatus{Ready: true}, nil
}

func (callbackExportTransfer) Transfer(ctx context.Context, request ExportTransferRequest) (ExportTransferOutcome, error) {
	_, _, err := request.PrepareFile(ctx)
	if err != nil {
		return ExportTransferOutcome{}, fmt.Errorf("record intent: %w", err)
	}
	return ExportTransferOutcome{}, nil
}

type failingTimeExport struct{ err error }

func (f failingTimeExport) ExportStaffTime(context.Context, workforce.TimeExportRequest, int64, string) (*workforce.ExportFile, error) {
	return nil, f.err
}

func (f failingTimeExport) DatevReport(context.Context, workforce.TimeExportRequest) (*workforce.DatevExportReport, error) {
	return nil, f.err
}
