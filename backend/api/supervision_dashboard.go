package api

// The root binds the supervision dashboard's access, settings and yard ports
// (#2703) to their owners: the Security Runtime for the overview and
// permission rules, the Settings Platform for the tenant settings, and
// Facilities for the Schulhof status. The dashboard names none of these
// owners (#4077); the translation is cross-owner glue and lives here. The
// session, schedule, presence and planning adapters stay in
// modules/supervisiondashboard/legacy.

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsplatform "github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
	supervisiondashboardCompose "github.com/moto-nrw/project-phoenix/modules/supervisiondashboard/compose"
)

// SupervisionCallerContext is the Identity & Access caller context the
// supervision dashboard reads: the access facts the root binds and the
// groups the dashboard's adapters list.
type SupervisionCallerContext interface {
	CurrentStaffID(context.Context) (*int64, error)
	FullStudentAccess(context.Context) (bool, error)
	HasCurrentStaff(context.Context) (bool, error)
	GetMySupervisedGroups(context.Context) ([]*studentpresence.SessionDetail, error)
	MyGroups(context.Context) ([]supervisiondashboardCompose.CallerGroup, error)
}

// supervisionAccessCaller is the slice of the caller context the access
// port reads.
type supervisionAccessCaller interface {
	CurrentStaffID(context.Context) (*int64, error)
	FullStudentAccess(context.Context) (bool, error)
	HasCurrentStaff(context.Context) (bool, error)
}

// newSupervisionAccess binds the dashboard's access port; without one of its
// sources it binds none, so the composition fails on the missing wiring.
func newSupervisionAccess(settings supervisiondashboard.Resolver, caller supervisionAccessCaller) supervisiondashboard.Access {
	if settings == nil || caller == nil {
		return nil
	}
	return supervisionAccess{settings: settings, caller: caller}
}

type supervisionAccess struct {
	settings supervisiondashboard.Resolver
	caller   supervisionAccessCaller
}

func (a supervisionAccess) CurrentStaffID(ctx context.Context) (*int64, error) {
	return a.caller.CurrentStaffID(ctx)
}

func (a supervisionAccess) FullStudentAccess(ctx context.Context) (bool, error) {
	return a.caller.FullStudentAccess(ctx)
}

// Caller asks the one school-wide overview rule (#2380). The organisational
// group mode is deliberately not consulted here: it describes how the school
// organises children, not who may open a running module.
func (a supervisionAccess) Caller(ctx context.Context) (supervisiondashboard.Caller, error) {
	principal, principalErr := permissions.PrincipalFromContext(ctx)
	assignmentBound := principalErr == nil && principal.Scope() == permissions.ScopeSchool
	admin := principalErr == nil && principal.HasAdminScope()
	overview, err := securityruntime.CanViewOperationalOverview(ctx, a.settings, a.caller, assignmentBound, admin)
	if err != nil {
		return supervisiondashboard.Caller{}, fmt.Errorf("resolve operational overview scope: %w", err)
	}
	claims := sessiontoken.ClaimsFromCtx(ctx)
	userPermissions := sessiontoken.PermissionsFromCtx(ctx)
	return supervisiondashboard.Caller{
		AccountID:           int64(claims.ID),
		TokenAdmin:          claims.IsAdmin,
		AdminScope:          admin,
		OperationalOverview: overview,
		CanReadSchedules:    securityruntime.HasPermission(permissions.SchedulesRead, userPermissions),
		CanReadStudents:     securityruntime.HasPermission(permissions.UsersRead, userPermissions),
	}, nil
}

// newSupervisionSettings binds the dashboard's settings port; without the
// resolver it binds none.
func newSupervisionSettings(settings supervisiondashboard.Resolver) supervisiondashboard.Settings {
	if settings == nil {
		return nil
	}
	return supervisionSettings{settings: settings}
}

type supervisionSettings struct{ settings supervisiondashboard.Resolver }

func (s supervisionSettings) Prepare(ctx context.Context) (context.Context, error) {
	return s.settings.Prefetch(ctx, []string{
		settingsplatform.KeyOperationalOverviewScope,
		settingsplatform.KeyGroupMode,
		settingsplatform.KeyStudentPhotosEnabled,
		settingsplatform.KeyWebSpontaneousActivities, settingsplatform.KeyWeekendFollowsFriday,
		settingsplatform.KeyTrackingIndicatorsEnabled,
		settingsplatform.KeyTrackingIndicator1,
		settingsplatform.KeyTrackingIndicator2,
		settingsplatform.KeyTrackingIndicator3,
	})
}

func (s supervisionSettings) StudentPhotosEnabled(ctx context.Context) (bool, error) {
	return s.settings.ResolveBool(ctx, settingsplatform.KeyStudentPhotosEnabled)
}

func (s supervisionSettings) TrackingIndicatorLabels(ctx context.Context) ([]string, error) {
	enabled, err := s.settings.ResolveBool(ctx, settingsplatform.KeyTrackingIndicatorsEnabled)
	if err != nil || !enabled {
		return nil, err
	}
	labels := make([]string, 0, 3)
	for _, key := range []string{settingsplatform.KeyTrackingIndicator1, settingsplatform.KeyTrackingIndicator2, settingsplatform.KeyTrackingIndicator3} {
		value, err := s.settings.ResolveString(ctx, key)
		if err != nil {
			return nil, err
		}
		if value = strings.TrimSpace(value); value != "" {
			labels = append(labels, value)
		}
	}
	return labels, nil
}

func (s supervisionSettings) SpontaneousActivitiesEnabled(ctx context.Context) (bool, error) {
	enabled, err := s.settings.ResolveBool(ctx, settingsplatform.KeyWebSpontaneousActivities)
	if err != nil {
		return false, fmt.Errorf("resolve spontaneous activities setting: %w", err)
	}
	return enabled, nil
}

// newSupervisionYard adapts the Facilities Schulhof service to the
// dashboard's yard port, also for callers that only need the courtyard
// status; without the service it binds none.
func newSupervisionYard(schulhof facilitiesCompose.SchulhofService) supervisiondashboard.Yard {
	if schulhof == nil {
		return nil
	}
	return supervisionYard{schulhof: schulhof}
}

type supervisionYard struct {
	schulhof facilitiesCompose.SchulhofService
}

// Status carries the Facilities status onto the dashboard field for field;
// the two shapes are the same wire contract.
func (y supervisionYard) Status(ctx context.Context, staffID int64) (*supervisiondashboard.SchulhofStatus, error) {
	status, err := y.schulhof.GetSchulhofStatus(ctx, staffID)
	if err != nil || status == nil {
		return nil, err
	}
	result := &supervisiondashboard.SchulhofStatus{
		Exists:            status.Exists,
		RoomID:            status.RoomID,
		RoomName:          status.RoomName,
		ActivityGroupID:   status.ActivityGroupID,
		ActiveGroupID:     status.ActiveGroupID,
		IsUserSupervising: status.IsUserSupervising,
		SupervisionID:     status.SupervisionID,
		SupervisorCount:   status.SupervisorCount,
		StudentCount:      status.StudentCount,
	}
	// A nil list stays nil, so the wire field serialises exactly like the
	// Facilities status.
	if status.Supervisors != nil {
		result.Supervisors = make([]supervisiondashboard.Supervisor, 0, len(status.Supervisors))
		for _, supervisor := range status.Supervisors {
			result.Supervisors = append(result.Supervisors, supervisiondashboard.Supervisor{
				ID: supervisor.ID, StaffID: supervisor.StaffID, Name: supervisor.Name, IsCurrentUser: supervisor.IsCurrentUser,
			})
		}
	}
	return result, nil
}
