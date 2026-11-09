package application

import (
	"context"
	"errors"
	"fmt"
	"slices"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
)

// weeklyPlanGroup is one connected companion graph. Even a child without a
// due plan can connect two due plans whose combined removals must be checked.
type weeklyPlanGroup struct {
	plans      []enrollmentNative.WeeklyPlan
	studentIDs []int64
}

func (d *Decisions) weeklyPlanGroups(ctx context.Context, due []enrollmentNative.WeeklyPlan) ([]weeklyPlanGroup, error) {
	groups := make([]weeklyPlanGroup, 0, len(due))
	byStudent := make(map[int64]int)
	for _, plan := range due {
		child, err := d.deps.Children.ChildByID(ctx, plan.RequestChildID)
		if err != nil {
			return nil, fmt.Errorf("weekly plan: load group child: %w", err)
		}
		if child == nil || child.CreatedStudentID == nil || d.deps.Companions == nil {
			groups = append(groups, weeklyPlanGroup{plans: []enrollmentNative.WeeklyPlan{plan}})
			continue
		}
		id := *child.CreatedStudentID
		index, found := byStudent[id]
		if !found {
			ids, err := d.weeklyPlanCompanionComponent(ctx, id)
			if err != nil {
				return nil, err
			}
			index = len(groups)
			groups = append(groups, weeklyPlanGroup{studentIDs: ids})
			for _, member := range ids {
				byStudent[member] = index
			}
		}
		groups[index].plans = append(groups[index].plans, plan)
	}
	return groups, nil
}

func (d *Decisions) weeklyPlanCompanionComponent(ctx context.Context, id int64) ([]int64, error) {
	ids := []int64{id}
	seen := map[int64]bool{id: true}
	for i := 0; i < len(ids); i++ {
		edges, err := d.deps.Companions.CompanionsOfStudent(ctx, ids[i])
		if err != nil {
			return nil, fmt.Errorf("weekly plan: read companion graph: %w", err)
		}
		for _, edge := range edges {
			far, ok := edge.other(ids[i])
			if ok && !seen[far] {
				seen[far] = true
				ids = append(ids, far)
			}
		}
	}
	slices.Sort(ids)
	return ids, nil
}

// applyWeeklyPlanGroup defers stranding verdicts only within a shared
// savepoint. The owner's final check runs before that savepoint is released.
func (d *Decisions) applyWeeklyPlanGroup(ctx context.Context, group weeklyPlanGroup) (int, error) {
	if len(group.plans) > 1 {
		if !d.deps.Runtime.InTransaction(ctx) {
			return 0, errors.New("weekly plan: companion group requires a transaction")
		}
		if d.deps.People.CompanionStrandings == nil {
			return 0, errors.New("weekly plan: companion stranding verifier is required")
		}
		if _, err := d.lockCompanionFarEnds(ctx, 0, group.studentIDs); err != nil {
			return 0, fmt.Errorf("weekly plan: lock companion graph: %w", err)
		}
		ctx, _ = departure.ContextWithStrandingBatch(ctx)
	}
	applied := 0
	for _, plan := range group.plans {
		ok, err := d.applyWeeklyPlan(ctx, plan)
		if err != nil {
			return 0, err
		}
		if ok {
			applied++
		}
	}
	if len(group.plans) > 1 {
		if err := d.deps.People.CompanionStrandings.VerifyCompanionStrandingBatch(ctx); err != nil {
			return 0, fmt.Errorf("weekly plan: verify companion links: %w", err)
		}
	}
	return applied, nil
}
