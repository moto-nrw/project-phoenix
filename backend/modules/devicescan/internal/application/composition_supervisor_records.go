package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/ports"
)

func LifecycleSupervisorsBinding(ctx context.Context, source devicescan.SupervisionQuery, sessionID int64) ([]ports.LifecycleSupervisor, error) {
	rows, err := source.ActiveSupervisions(ctx, sessionID)
	if err != nil {
		return []ports.LifecycleSupervisor{}, err
	}
	result := make([]ports.LifecycleSupervisor, 0, len(rows))
	for _, row := range rows {
		result = append(result, ports.LifecycleSupervisor{StaffID: row.StaffID, Role: row.Role, Ended: row.Ended})
	}
	return result, nil
}
