package calendar_test

import (
	"context"

	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	"github.com/moto-nrw/project-phoenix/modules/appointments"
)

func (failingCandidateLock) FindReminderCandidatesForUpdate(context.Context, []int64) ([]*appointments.Appointment, error) {
	return nil, errReminderStore
}

func (failingRecurrenceReload) FindByAppointmentIDs(context.Context, []int64) ([]*calModels.RecurrenceRule, error) {
	return nil, errReminderStore
}

func (failingRecipientLookup) FindAppointmentRecipientsByAppointmentIDs(context.Context, []int64) ([]*appointments.AppointmentRecipient, error) {
	return nil, errReminderStore
}

func (r *revisingCandidateLock) FindReminderCandidatesForUpdate(ctx context.Context, ids []int64) ([]*appointments.Appointment, error) {
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
	return r.Capability.FindReminderCandidatesForUpdate(ctx, ids)
}
