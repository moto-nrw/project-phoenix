package application

import (
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

func monthSnapshotsToPublic(values []*domain.StaffMonthBalanceSnapshot) []*workforce.StaffMonthBalanceSnapshot {
	if values == nil {
		return nil
	}
	result := make([]*workforce.StaffMonthBalanceSnapshot, 0, len(values))
	for _, value := range values {
		converted := workforce.StaffMonthBalanceSnapshot(*value)
		result = append(result, &converted)
	}
	return result
}
