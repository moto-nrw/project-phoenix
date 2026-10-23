package staffoffboarding

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// StaffBirthdayWrites preserves the live-membership guard and the employment
// write, including their retained error mapping, at the root binding.
type StaffBirthdayWrites interface {
	SetBirthdayDisplayOptOut(context.Context, int64, bool) error
}

// BirthdayPreferences is the HTTP workflow entry. The domain supplies only
// reads; no People Directory application calls back into this workflow.
type BirthdayPreferences struct {
	peopledirectory.Birthdays
	writes StaffBirthdayWrites
	logger *slog.Logger
}

func NewBirthdayPreferences(reads peopledirectory.Birthdays, writes StaffBirthdayWrites, logger *slog.Logger) *BirthdayPreferences {
	if reads == nil || writes == nil {
		panic("staff birthday preferences: reads and writes are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &BirthdayPreferences{Birthdays: reads, writes: writes, logger: logger}
}

// SetOptOut resolves only the caller's staff row, preserves the unchanged-value
// no-op and writes inside the existing membership-before-employment protocol.
// Every operation joins the caller's tenant transaction, as before.
func (w *BirthdayPreferences) SetOptOut(ctx context.Context, accountID int64, optOut bool) error {
	staff, err := w.OwnStaffBirthday(ctx, accountID)
	if err != nil {
		return err
	}
	if staff.OptOut == optOut {
		return nil
	}
	if err := w.writes.SetBirthdayDisplayOptOut(ctx, staff.StaffID, optOut); err != nil {
		return fmt.Errorf("users.set birthday display opt-out: %w", err)
	}
	w.logger.Debug("birthday display opt-out changed",
		"staff_id", staff.StaffID,
		"opt_out", optOut,
	)
	return nil
}
