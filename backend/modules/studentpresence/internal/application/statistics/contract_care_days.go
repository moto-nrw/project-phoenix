package statistics

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

// WithContractCareDays lets the care contract decide the Soll on the days it
// covers (#4683, ADR 0156): its modules say whether the day is booked and
// the latest module end is the pickup. The contract names no arrival, so
// the care plan's arrival and a cancellation of the day ("Kommt heute
// nicht") stay. Without contracts the care plan decides alone.
func WithContractCareDays(plan ports.PlannedCareDays, contracts ports.ContractCareDays) ports.PlannedCareDays {
	if plan == nil || contracts == nil {
		return plan
	}
	return contractCareDays{plan: plan, contracts: contracts}
}

type contractCareDays struct {
	plan      ports.PlannedCareDays
	contracts ports.ContractCareDays
}

func (c contractCareDays) PlannedCareDays(ctx context.Context, studentIDs []int64, from, to ports.Date) (map[int64]map[ports.Date]studentpresence.PlannedCareDay, error) {
	planned, err := c.plan.PlannedCareDays(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}
	contracts, err := c.contracts.ContractCareDays(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}
	for studentID, days := range contracts {
		if planned[studentID] == nil {
			planned[studentID] = map[ports.Date]studentpresence.PlannedCareDay{}
		}
		for date, booked := range days {
			planned[studentID][date] = contractCareDay(booked, planned[studentID][date])
		}
	}
	return planned, nil
}

// contractCareDay is the Soll of a day the contract covers, over the care
// plan's verdict of that day.
func contractCareDay(booked studentpresence.ContractCareDay, plan studentpresence.PlannedCareDay) studentpresence.PlannedCareDay {
	// A cancellation of the day stays, also on a day the contract books no
	// module: a holiday booking above must not undo it (#5994).
	day := studentpresence.PlannedCareDay{Source: studentpresence.PlannedCareSourceContract, Cancelled: plan.Cancelled}
	if !booked.Booked {
		return day
	}
	day.Expected, day.Cancelled = !plan.Cancelled, plan.Cancelled
	day.Arrival, day.Pickup = plan.Arrival, booked.Pickup
	return day
}
