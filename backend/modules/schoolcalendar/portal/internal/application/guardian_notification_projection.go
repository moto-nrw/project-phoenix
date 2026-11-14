package application

import (
	"context"
	"maps"

	appointmentcap "github.com/moto-ogs/project-phoenix/modules/appointments"
)

// GuardianNotificationProfile contains only delivery identity and display data.
// Permission checks happen before this projection is returned.
type GuardianNotificationProfile struct {
	ID           int64
	AccountID    *int64
	Email        *string
	FirstName    string
	LastName     string
	PortalLocale *string
}

// GuardianNotificationAudience records the relationship-level access checked for
// one appointment. GuardianIDs preserves recipient order, including duplicates.
type GuardianNotificationAudience struct {
	GuardianIDs        []int64
	Profiles           map[int64]*GuardianNotificationProfile
	StudentsByGuardian map[int64][]int64
	// Answers holds each guardian's answer to the appointment, so reminders can
	// leave out the dates a guardian declined (#7014).
	Answers map[int64]GuardianNotificationAnswer
}

// GuardianNotificationAnswer is a guardian's answer to an appointment: the
// answer for the whole appointment or series and the dates of a series that
// were answered on their own (#6987).
type GuardianNotificationAnswer struct {
	Status              string
	OccurrenceResponses map[appointmentcap.Date]string
}

// GuardianNotificationAudiences is the tenant-safe notification read projection.
// It reuses the lifecycle path's active portal and parent-child permission checks.
func (s *service) GuardianNotificationAudiences(ctx context.Context, appointmentIDs []int64) (map[int64]GuardianNotificationAudience, error) {
	values, err := s.reachableGuardianRecipientsByAppointment(ctx, appointmentIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]GuardianNotificationAudience, len(values))
	for appointmentID, value := range values {
		audience := GuardianNotificationAudience{
			GuardianIDs:        append([]int64(nil), value.guardianIDs...),
			Profiles:           make(map[int64]*GuardianNotificationProfile, len(value.profiles)),
			StudentsByGuardian: make(map[int64][]int64, len(value.studentsByGuardian)),
			Answers:            make(map[int64]GuardianNotificationAnswer, len(value.answers)),
		}
		for id, answer := range value.answers {
			audience.Answers[id] = GuardianNotificationAnswer{Status: answer.Status, OccurrenceResponses: maps.Clone(answer.OccurrenceResponses)}
		}
		for id, profile := range value.profiles {
			if profile == nil {
				continue
			}
			audience.Profiles[id] = &GuardianNotificationProfile{
				ID: profile.ID, AccountID: profile.AccountID, Email: profile.Email,
				FirstName: profile.FirstName, LastName: profile.LastName, PortalLocale: profile.PortalLocale,
			}
		}
		for id, students := range value.studentsByGuardian {
			audience.StudentsByGuardian[id] = append([]int64(nil), students...)
		}
		result[appointmentID] = audience
	}
	return result, nil
}
