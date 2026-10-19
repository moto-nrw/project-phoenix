package timetracking

import (
	"context"
)

func (s *monthCloseCommands) CloseMonth(ctx context.Context, closedBy int64, year, month int, reason string) (value0 *MonthCloseResult, err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.CloseMonth(txCtx, closedBy, year, month, reason)
		return err
	})
	return
}

func (s *monthCloseCommands) ReopenMonth(ctx context.Context, staffID, reopenedBy int64, year, month int, reason string) (err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		err = s.commands.ReopenMonth(txCtx, staffID, reopenedBy, year, month, reason)
		return err
	})
	return
}

func (s *monthCloseCommands) ListMonthStatus(ctx context.Context, year, month int) (value0 []*MonthSnapshot, err error) {
	err = s.commands.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.commands.ListMonthStatus(txCtx, year, month)
		return err
	})
	return
}
