package api

import (
	"context"
	"fmt"
)

type seedStudentStatusVariantsStep struct{}

func (seedStudentStatusVariantsStep) Name() string { return "Seeding student absence variants" }

func (seedStudentStatusVariantsStep) Run(_ context.Context, rt *Runtime) error {
	if rt.FixedSeeder == nil {
		return fmt.Errorf("student absence prerequisites not available")
	}
	variants := []struct {
		index  int
		status string
		days   int
	}{
		{index: 0, status: "excused", days: 1},
		{index: 1, status: "class_trip", days: 2},
		{index: 2, status: "sick", days: 3},
	}
	for _, variant := range variants {
		studentID := rt.FixedSeeder.studentIDByIndex[variant.index]
		if studentID == 0 {
			return fmt.Errorf("student absence demo index %d not available", variant.index)
		}
		date := todaySeedDate().AddDays(variant.days).String()
		if _, err := rt.Client.Post(fmt.Sprintf("/api/students/%d/status-days", studentID), map[string]any{
			"status": variant.status, "dates": []string{date}, "reason": "Demo-Planung",
		}); err != nil {
			return fmt.Errorf("seed %s status day: %w", variant.status, err)
		}
	}
	return nil
}

type seedInactiveAccountStep struct{}

func (seedInactiveAccountStep) Name() string { return "Seeding inactive account variant" }

func (seedInactiveAccountStep) Run(_ context.Context, rt *Runtime) error {
	if rt.FixedSeeder == nil || len(rt.FixedSeeder.staffCredentials) == 0 {
		return fmt.Errorf("inactive account prerequisites not available")
	}
	rt.Client.BindAuth(rt.TenantAuth)
	guest := rt.FixedSeeder.staffCredentials[len(rt.FixedSeeder.staffCredentials)-1]
	staffID := rt.FixedSeeder.staffIDs[guest.Name]
	if staffID == 0 {
		return fmt.Errorf("demo guest staff record not available")
	}
	if _, err := rt.Client.Delete(fmt.Sprintf("/api/staff/%d", staffID)); err != nil {
		return fmt.Errorf("offboard demo guest: %w", err)
	}
	return nil
}
