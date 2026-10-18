package api

import (
	"context"

	calendarService "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
)

type CalendarReminderSource interface {
	ReminderEffects() calendarService.ReminderEffects
}

// CalendarReminderDelivery binds the existing notification adapters of the
// calendar to the reminder workflow's delivery port.
func CalendarReminderDelivery(source CalendarReminderSource) ports.DeliveryDependencies {
	effects := source.ReminderEffects()
	return ports.DeliveryDependencies{
		Now:          effects.Now,
		Appointments: effects.Appointments, Email: effects.Email, Push: effects.Push,
		FilterEmail: effects.FilterEmail, FilterPush: effects.FilterPush, Brand: effects.Brand,
		WhenText: effects.WhenText, NoDelivery: effects.NoDelivery, ParentsURL: effects.ParentsURL, Logger: effects.Logger,
		Audiences: func(ctx context.Context, ids []int64) (map[int64]ports.GuardianAudience, error) {
			values, err := effects.Audiences(ctx, ids)
			if err != nil {
				return nil, err
			}
			result := make(map[int64]ports.GuardianAudience, len(values))
			for id, value := range values {
				audience := ports.GuardianAudience{GuardianIDs: value.GuardianIDs, StudentsByGuardian: value.StudentsByGuardian, Profiles: make(map[int64]*ports.GuardianProfile, len(value.Profiles))}
				for profileID, profile := range value.Profiles {
					if profile == nil {
						continue
					}
					audience.Profiles[profileID] = &ports.GuardianProfile{ID: profile.ID, AccountID: profile.AccountID, Email: profile.Email, FirstName: profile.FirstName, LastName: profile.LastName, PortalLocale: profile.PortalLocale}
				}
				result[id] = audience
			}
			return result, nil
		},
	}
}
