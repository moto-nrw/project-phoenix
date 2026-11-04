package ports

import "context"

// HolidayGuestReconciliation brings the holiday guests of the school in ctx
// in line with the confirmed bookings of the weeks it hosts (#4666,
// ADR 0120): new guests join the group the school picked, cancelled ones
// leave it, and their contacts go seven days after the week. The root binds
// Enrollment's holiday guests.
type HolidayGuestReconciliation func(ctx context.Context) error
