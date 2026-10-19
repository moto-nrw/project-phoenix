package timetracking

import (
	"context"
)

func (s *monthCloseCommands) CloseMonth(ctx context.Context, closedBy int64, year, month int, reason string) (value0 *MonthCloseResult, err error) {
	err = s.staffMonthCloseService.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.staffMonthCloseService.CloseMonth(txCtx, closedBy, year, month, reason)
		return err
	})
	return
}

func (s *monthCloseCommands) ReopenMonth(ctx context.Context, staffID, reopenedBy int64, year, month int, reason string) (err error) {
	err = s.staffMonthCloseService.runCommand(ctx, func(txCtx context.Context) error {
		err = s.staffMonthCloseService.ReopenMonth(txCtx, staffID, reopenedBy, year, month, reason)
		return err
	})
	return
}

func (s *monthCloseCommands) ListMonthStatus(ctx context.Context, year, month int) (value0 []*MonthSnapshot, err error) {
	err = s.staffMonthCloseService.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.staffMonthCloseService.ListMonthStatus(txCtx, year, month)
		return err
	})
	return
}
