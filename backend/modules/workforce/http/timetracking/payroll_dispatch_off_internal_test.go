package timetracking

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
)

// payrollDispatchOff is a school without the automatic send of #4805: the
// month close queues nothing.
type payrollDispatchOff struct{}

func (payrollDispatchOff) QueueAfterMonthClose(context.Context, workforce.PayrollDispatchTrigger) error {
	return nil
}

func (payrollDispatchOff) Overview(context.Context) (workforce.PayrollDispatchOverview, error) {
	return workforce.PayrollDispatchOverview{}, nil
}

func (payrollDispatchOff) RunDue(context.Context) (int, error) { return 0, nil }

func (payrollDispatchOff) Download(context.Context, int64, string) (workforce.PayrollDownload, error) {
	return workforce.PayrollDownload{}, workforce.ErrPayrollDownloadUnavailable
}
