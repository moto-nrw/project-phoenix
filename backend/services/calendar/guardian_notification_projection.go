package calendar

import "context"

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
