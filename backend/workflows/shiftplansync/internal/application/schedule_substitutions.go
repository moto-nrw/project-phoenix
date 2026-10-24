package application

import (
	"context"

	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/contract"
)

// ScheduleRuntime is the substitution request's tenant transaction.
type ScheduleRuntime interface {
	// RunInTx joins the ambient tenant transaction or opens one for the
	// context's school.
	RunInTx(ctx context.Context, fn func(context.Context) error) error
	// TenantID is the school the caller acts for, 0 when none.
	TenantID(ctx context.Context) int64
	// MarkRollback asks the request's tenant transaction to roll back.
	MarkRollback(ctx context.Context)
}

// ScheduleSubstitutions is the Terminvertretung entry of /api/substitutions
// (#4825): it authorizes the caller, runs the Timetable deviation writes in
// the request's tenant transaction and announces them after commit. School
// Structure's substitution module serves the group targets and no longer
// calls this workflow; the root routes the schedule target here.
type ScheduleSubstitutions struct {
	schedule *SubstitutionAdapter
	runtime  ScheduleRuntime
}

// NewScheduleSubstitutions binds the entry to the schedule writes and the
// request runtime.
func NewScheduleSubstitutions(schedule *SubstitutionAdapter, runtime ScheduleRuntime) *ScheduleSubstitutions {
	return &ScheduleSubstitutions{schedule: schedule, runtime: runtime}
}

// Overview lists the period's changed appointments and, on request, the
// staff a planner may pick.
func (s *ScheduleSubstitutions) Overview(ctx context.Context, caller education.Caller, query education.OverviewQuery) (*education.OverviewResult, error) {
	if query.ScheduleFrom == nil || query.ScheduleTo == nil {
		return nil, education.ErrInvalidPeriod
	}
	if err := s.authorize(ctx, caller, "schedules:read"); err != nil {
		return nil, err
	}
	overview, err := s.schedule.Overview(
		ctx, *query.ScheduleFrom, *query.ScheduleTo, query.IncludeScheduleTargets,
		caller.HasPermission("schedules:manage"),
	)
	if err != nil {
		return nil, err
	}
	return &education.OverviewResult{
		GroupHandovers: []education.GroupHandover{}, Groups: []education.GroupRef{}, Targets: []education.StaffRef{},
		RunningSupervisions:  []education.RunningSupervision{},
		ScheduleAppointments: overview.Appointments, ScheduleTargets: overview.Targets,
	}, nil
}

// Assign applies one schedule substitution. A refused write rolls the
// request's transaction back.
func (s *ScheduleSubstitutions) Assign(ctx context.Context, caller education.Caller, assignment education.Assignment) (result *education.AssignmentResult, err error) {
	err = s.runtime.RunInTx(ctx, func(ctx context.Context) error {
		if assignment.Type != education.TargetScheduleSubstitution || assignment.ScheduleSubstitution == nil {
			return education.ErrInvalidTarget
		}
		if err := s.authorize(ctx, caller, "schedules:manage"); err != nil {
			return err
		}
		written, err := s.schedule.Assign(ctx, *assignment.ScheduleSubstitution, caller.AccountID)
		if err != nil {
			s.runtime.MarkRollback(ctx)
			return err
		}
		afterScheduleChange(ctx, written)
		result = &education.AssignmentResult{ScheduleSubstitution: written}
		return nil
	})
	return result, err
}

// End ends one running schedule substitution.
func (s *ScheduleSubstitutions) End(ctx context.Context, caller education.Caller, request education.EndRequest) error {
	return s.runtime.RunInTx(ctx, func(ctx context.Context) error {
		if request.Type != education.TargetScheduleSubstitution || request.ID <= 0 {
			return education.ErrInvalidTarget
		}
		if err := s.authorize(ctx, caller, "schedules:manage"); err != nil {
			return err
		}
		written, err := s.schedule.End(ctx, request.ID, caller.AccountID)
		if err != nil {
			s.runtime.MarkRollback(ctx)
			return err
		}
		afterScheduleChange(ctx, written)
		return nil
	})
}

// authorize admits an org-scoped caller of the request's school holding the
// required schedule permission.
func (s *ScheduleSubstitutions) authorize(ctx context.Context, caller education.Caller, required string) error {
	if caller.AccountID <= 0 || caller.TenantID <= 0 || caller.TenantID != s.runtime.TenantID(ctx) ||
		(caller.Scope != "" && caller.Scope != "org") {
		return education.ErrForbidden
	}
	if caller.HasPermission == nil || !caller.HasPermission(required) {
		return education.ErrForbidden
	}
	return nil
}

func afterScheduleChange(ctx context.Context, result *education.ScheduleSubstitutionResult) {
	if result != nil && result.AfterCommit != nil {
		result.AfterCommit(ctx)
	}
}
