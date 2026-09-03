package calendar_test

import (
	"context"

	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
)

func (failingCandidateLock) LockReminderCandidates(context.Context, []int64) ([]*calModels.Appointment, error) {
	return nil, errReminderStore
}

func (failingRecurrenceReload) FindByAppointmentIDs(context.Context, []int64) ([]*calModels.RecurrenceRule, error) {
	return nil, errReminderStore
}

func (failingRecipientLookup) FindByAppointmentIDs(context.Context, []int64) ([]*calModels.AppointmentRecipient, error) {
	return nil, errReminderStore
}

func (r *revisingCandidateLock) LockReminderCandidates(ctx context.Context, ids []int64) ([]*calModels.Appointment, error) {
	r.mu.Lock()
	r.calls++
	revise := r.calls == r.on
	r.mu.Unlock()
	if revise && len(ids) > 0 {
		if _, err := r.db.ExecContext(context.Background(),
			`UPDATE calendar.appointments SET revision = revision + 1 WHERE id = ?`, ids[0]); err != nil {
			return nil, err
		}
	}
	return r.AppointmentRepository.LockReminderCandidates(ctx, ids)
}
