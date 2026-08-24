package users

import (
	"context"
	"time"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/config/sideeffects"
)

// RegisterCareWithdrawalSettingsSideEffects keeps pending booking-derived
// completion tasks from surviving a switch to weekly-plan-driven care.
func RegisterCareWithdrawalSettingsSideEffects(registry *sideeffects.Registry, repo userModels.CareWithdrawalCompletionRepository) {
	registry.Register(configModel.KeyEnrollmentBookingsAuthoritative, func(ctx context.Context, _ int64, value any) (func(), error) {
		authoritative, ok := value.(bool)
		if !ok || authoritative {
			return nil, nil
		}
		_, err := repo.MarkPendingObsoleteForWeeklyPlans(ctx, time.Now())
		return nil, err
	})
}
