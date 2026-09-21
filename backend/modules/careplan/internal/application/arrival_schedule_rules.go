package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

type arrivalScheduleRules struct {
	students ports.ArrivalRuleStudents
	classes  ports.ArrivalClassPlans
}

func NewArrivalScheduleRules(students ports.ArrivalRuleStudents, classes ports.ArrivalClassPlans) ports.ArrivalScheduleRules {
	if students == nil {
		return nil
	}
	return arrivalScheduleRules{students, classes}
}

func (s arrivalScheduleRules) LockStudent(ctx context.Context, id int64) error {
	return s.students.LockStudent(ctx, id)
}

func (s arrivalScheduleRules) ClassTimesForStudent(ctx context.Context, id int64) (map[int]time.Time, error) {
	if s.classes == nil {
		return nil, nil
	}
	class, err := s.students.SchoolClass(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load student before checking class arrival time: %w", err)
	}
	if strings.TrimSpace(class) == "" {
		return nil, nil
	}
	rows, err := s.classes.FindByClasses(ctx, []string{class})
	if err != nil {
		return nil, fmt.Errorf("load class arrival time before saving weekly plan: %w", err)
	}
	if len(rows) == 0 || rows[0] == nil {
		return nil, nil
	}
	return domain.ClassArrivalBaselineFromTimes(rows[0].SchoolClass, rows[0].ArrivalTimes).ArrivalTimes, nil
}
