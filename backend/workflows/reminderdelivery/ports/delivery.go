package ports

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
)

type GuardianProfile struct {
	ID           int64
	AccountID    *int64
	Email        *string
	FirstName    string
	LastName     string
	PortalLocale *string
}

// StudentIDs restricts the checked child scope to the accounts being addressed.
func (a GuardianAudience) StudentIDs(accountIDs []int64) []int64 {
	addressed := make(map[int64]struct{}, len(accountIDs))
	for _, id := range accountIDs {
		addressed[id] = struct{}{}
	}
	result := make([]int64, 0, len(a.StudentsByGuardian))
	seen := make(map[int64]struct{})
	for _, id := range a.GuardianIDs {
		profile := a.Profiles[id]
		if profile == nil || profile.AccountID == nil {
			continue
		}
		if _, ok := addressed[*profile.AccountID]; !ok {
			continue
		}
		for _, studentID := range a.StudentsByGuardian[id] {
			if _, ok := seen[studentID]; ok {
				continue
			}
			seen[studentID] = struct{}{}
			result = append(result, studentID)
		}
	}
	return result
}

// GuardianAudience contains only relationships already checked for portal access.
type GuardianAudience struct {
	GuardianIDs        []int64
	Profiles           map[int64]*GuardianProfile
	StudentsByGuardian map[int64][]int64
}

type DeliveryDependencies struct {
	Appointments appointments.Capability
	Audiences    func(context.Context, []int64) (map[int64]GuardianAudience, error)
	Email        func(context.Context, string, int64, map[string]any) (bool, error)
	Push         func(context.Context, int64, []int64, []int64, string) (bool, error)
	FilterEmail  func(context.Context, []int64) ([]int64, error)
	FilterPush   func(context.Context, []int64) ([]int64, error)
	Brand        func(context.Context, int64) (string, string, string)
	WhenText     func(*appointments.Appointment) string
	NoDelivery   func(error) bool
	ParentsURL   string
	Logger       *slog.Logger
}
