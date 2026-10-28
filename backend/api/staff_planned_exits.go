package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/render"

	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	schoolMembershipModule "github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	staffHTTP "github.com/moto-nrw/project-phoenix/modules/schoolmembership/http"
	"github.com/moto-nrw/project-phoenix/observability"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/staffoffboarding"
)

// PlannedStaffExitsJob builds the Worker's planned staff exits (#4809) over
// the offboarding workflow the DELETE route uses, so a planned exit runs the
// same owners in the same order. The Worker has no request principal: the
// planner acts in the name of the person who planned the exit, and the
// request authorization of the workflow refuses everything else.
func PlannedStaffExitsJob(db transactionruntime.Database, hub *realtime.Hub, logger *slog.Logger) owners.PlannedStaffExitsBuilder {
	return func(identity identityaccess.Owner, notifications owners.DeliveryNotifications, staff owners.StaffPermissionHolders) schedulerPorts.PlannedStaffExits {
		if identity == nil || notifications.Notifier == nil || notifications.Preferences == nil || staff == nil {
			panic("planned staff exits: identity, notifications and staff are required")
		}
		offboarding, err := NewStaffOffboardingOwners(db, peopleCompose.MustNewReads(db.Pool(), studentDirectoryView.NewReader()), owners.MembershipStaffEmployment(workforceCompose.MustNewStaffEmployment(db.Pool())), logger)
		if err != nil {
			panic(err)
		}
		cleanup, err := workforceCompose.NewDocumentCleanup(db.Pool(), nil)
		if err != nil {
			panic(err)
		}
		workflow := newStaffOffboardingWorkflow(hub, identity, offboarding, logger, StaffMembershipHooks{
			ResolveEditorStaffID: func(context.Context) (int64, error) { return 0, staffoffboarding.ErrUnauthorized },
			// The Worker enqueues the document cleanup only; the existing
			// five-minute file cleanup picks it up (ADR 0013).
			QueueOffboardedDocumentCleanup: cleanup.Enqueue,
		})
		planner := newStaffExitPlanner(workflow, offboarding, staffExitNotifier(notifications, staff, logger))
		return planner.RunDue
	}
}

// staffExitNotifier tells the Leitung, the holders of users:delete who did
// not switch the notice off, about a planned exit. The push lands on a lock
// screen, so it names no person; the link opens the profile. A notice the
// school's settings hold back now is reported unsent and repeated later.
func staffExitNotifier(notifications owners.DeliveryNotifications, staff owners.StaffPermissionHolders, logger *slog.Logger) func(context.Context, staffoffboarding.ExitNotice) (bool, error) {
	return func(ctx context.Context, notice staffoffboarding.ExitNotice) (bool, error) {
		holders, err := staff.ListStaffWithPermission(ctx, permissions.UsersDelete)
		if err != nil {
			return false, fmt.Errorf("list staff exit recipients: %w", err)
		}
		candidates := staffAccountIDs(holders)
		if len(candidates) == 0 {
			return true, nil
		}
		recipients, err := notifications.Preferences.FilterNotOptedOut(ctx, deliveryContract.NotificationStaffPlannedExit, candidates)
		if err != nil {
			return false, fmt.Errorf("filter staff exit recipients: %w", err)
		}
		if len(recipients) == 0 {
			return true, nil
		}
		event, err := staffExitEvent(notice, recipients)
		if err != nil {
			return false, err
		}
		err = notifications.Notifier.Notify(ctx, event)
		if errors.Is(err, deliveryCompose.ErrNotificationsDisabled) || errors.Is(err, deliveryCompose.ErrOutsideNotificationWindow) {
			logger.Debug("planned staff exit notice held back",
				"staff_id", notice.StaffID,
				"kind", notice.Kind,
			)
			return false, nil
		}
		return err == nil, err
	}
}

func staffAccountIDs(holders []*peopleValues.StaffWithRoleInfo) []int64 {
	seen := make(map[int64]bool, len(holders))
	ids := make([]int64, 0, len(holders))
	for _, holder := range holders {
		if holder != nil && holder.AccountID > 0 && !seen[holder.AccountID] {
			seen[holder.AccountID] = true
			ids = append(ids, holder.AccountID)
		}
	}
	return ids
}

// staffExitEvent is the notice text of #4809 in moto's plain German.
func staffExitEvent(notice staffoffboarding.ExitNotice, recipients []int64) (deliveryCompose.NotificationEvent, error) {
	day, err := calendar.ParseDate(notice.LastWorkingDay)
	if err != nil {
		return deliveryCompose.NotificationEvent{}, err
	}
	var title, body string
	switch notice.Kind {
	case staffoffboarding.NoticeWeekBefore:
		title = "Austritt in einer Woche"
		body = "Am " + day.Format("02.01.2006") + " hat eine Person aus dem Team ihren letzten Arbeitstag. Danach endet ihr Zugang zu moto."
	case staffoffboarding.NoticeExitDay:
		title = "Heute letzter Arbeitstag"
		body = "Heute hat eine Person aus dem Team ihren letzten Arbeitstag. Heute Nacht endet ihr Zugang zu moto."
	case staffoffboarding.NoticeBlocked:
		title = "Austritt wartet noch"
		body = "Die Person führt noch eine Aufsicht oder hat eine Gruppe übernommen. Ihre Anmeldung ist schon gesperrt. Beenden Sie die Aufsicht oder Übernahme. moto versucht es jede Stunde wieder."
	default:
		return deliveryCompose.NotificationEvent{}, fmt.Errorf("unknown staff exit notice %q", notice.Kind)
	}
	return deliveryCompose.NotificationEvent{
		Type:           deliveryContract.NotificationStaffPlannedExit,
		IdempotencyKey: fmt.Sprintf("staff-exit-%d-%s-%s", notice.StaffID, notice.LastWorkingDay, notice.Kind),
		RelatedType:    "staff",
		RelatedID:      notice.StaffID,
		Title:          title,
		Body:           body,
		DeepLink:       fmt.Sprintf("/staff/%d", notice.StaffID),
		Audience: deliveryCompose.NotificationAudience{
			TenantID:        notice.TenantID,
			Scope:           deliveryCompose.NotificationAudienceScope("staff"),
			StaffAccountIDs: recipients,
		},
	}, nil
}

// renderStaffExitFailure answers the refusals of a planned exit with their
// own codes; the rest follows the staff write classification.
func renderStaffExitFailure(w http.ResponseWriter, r *http.Request, err error) {
	var (
		status   int
		renderer render.Renderer
	)
	switch {
	case errors.Is(err, staffoffboarding.ErrPlannedExitInPast):
		status, renderer = http.StatusBadRequest, apiCommon.ErrorInvalidRequestWithCode(err, apiCommon.CodeStaffExitDayInPast)
	case errors.Is(err, staffoffboarding.ErrPlannedExitStarted):
		status, renderer = http.StatusConflict, apiCommon.ErrorConflictWithCode(err, apiCommon.CodeStaffExitAlreadyDue)
	case errors.Is(err, schoolMembershipModule.ErrPlannedExitNotFound):
		status, renderer = http.StatusNotFound, apiCommon.ErrorNotFoundWithCode(err, apiCommon.CodeStaffExitNotPlanned)
	case errors.Is(err, schoolMembershipModule.ErrInvalidMembership):
		renderStaffFailure(w, r, staffHTTP.FailureInvalidRequest, err)
		return
	case errors.Is(err, schoolMembershipModule.ErrStaffNotFound):
		renderStaffFailure(w, r, staffHTTP.FailureNotFound, err)
		return
	default:
		renderStaffWriteFailure(w, r, err)
		return
	}
	observability.ObserveSchoolMembershipHTTPResponse(status, "planned_exit")
	apiCommon.RenderError(w, r, renderer)
}
