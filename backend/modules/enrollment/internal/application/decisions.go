package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// openSchoolClassPlaceholder satisfies the required users.students.school_class
// column when enrollment deliberately did not collect a grade or concrete
// class. It is application-owned state, not an administrator-assigned class.
const openSchoolClassPlaceholder = "offen"

var validDecisionStatuses = map[enrollment.DecisionStatus]bool{
	enrollment.DecisionApproved:    true,
	enrollment.DecisionWaitlisted:  true,
	enrollment.DecisionRejected:    true,
	enrollment.DecisionUnderReview: true,
}

// Decisions is Enrollment's admin decision flow: the review queue, the
// per-child decision with the downstream records of an approval, the
// restore of a withdrawn request, the booking corrections, the approved
// child sync of change requests and the audited exports.
type Decisions struct {
	deps     DecisionDependencies
	capacity *OfferingCapacity
}

var (
	_ enrollment.Decisions            = (*Decisions)(nil)
	_ enrollment.ApprovedChildChanges = (*Decisions)(nil)
)

// NewDecisions composes the decision flow over its owners.
func NewDecisions(deps DecisionDependencies) *Decisions {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.Today == nil {
		deps.Today = calendar.TodayDate
	}
	deps.ParentsURL = strings.TrimRight(strings.TrimSpace(deps.ParentsURL), "/")
	d := &Decisions{deps: deps}
	d.capacity = NewOfferingCapacity(deps.Offerings, deps.Children, func(ctx context.Context) (bool, error) {
		return d.waitlistEnabled(ctx)
	})
	return d
}

func (d *Decisions) todayDate() calendar.Date {
	return d.deps.Today()
}

func (d *Decisions) logger() *slog.Logger {
	return d.deps.Logger
}

// DecisionRequests lists the requests of the admin queue.
func (d *Decisions) DecisionRequests(ctx context.Context, filters enrollment.DecisionRequestFilters) ([]*enrollment.DecisionSummary, error) {
	return d.listDecisionRequests(ctx, enrollment.RequestListFilters{
		PhaseID:     filters.PhaseID,
		ChildStatus: filters.ChildStatus,
	}, "decision: list requests")
}

// StudentDecisionRequests lists the requests whose children created the
// student, keeping only those children.
func (d *Decisions) StudentDecisionRequests(ctx context.Context, studentID int64) ([]*enrollment.DecisionSummary, error) {
	if studentID <= 0 {
		return nil, fmt.Errorf("decision: student_id required")
	}
	return d.listDecisionRequests(ctx, enrollment.RequestListFilters{CreatedStudentID: studentID}, "decision: list requests by student")
}

// listDecisionRequests reads the admin requests the filters select and
// assembles them; a CreatedStudentID keeps only that student's children.
func (d *Decisions) listDecisionRequests(ctx context.Context, filters enrollment.RequestListFilters, errPrefix string) ([]*enrollment.DecisionSummary, error) {
	requests, err := d.deps.Requests.AdminRequests(ctx, filters)
	if err == nil {
		_, err = requestValues(requests)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errPrefix, err)
	}
	return d.assembleAll(ctx, requests, filters.CreatedStudentID)
}

// assembleAll assembles every request; a positive studentID keeps only the
// children that created that student and drops requests without one.
func (d *Decisions) assembleAll(ctx context.Context, requests []*enrollment.Request, studentID int64) ([]*enrollment.DecisionSummary, error) {
	out := make([]*enrollment.DecisionSummary, 0, len(requests))
	for _, req := range requests {
		summary, err := d.assemble(ctx, req)
		if err != nil {
			return nil, err
		}
		if studentID <= 0 {
			out = append(out, summary)
			continue
		}
		filterSummaryChildren(summary, studentID)
		if len(summary.Children) > 0 {
			out = append(out, summary)
		}
	}
	return out, nil
}

func filterSummaryChildren(summary *enrollment.DecisionSummary, studentID int64) {
	if summary == nil || studentID <= 0 {
		return
	}
	children := summary.Children[:0]
	for _, child := range summary.Children {
		if child != nil && child.CreatedStudentID != nil && *child.CreatedStudentID == studentID {
			children = append(children, child)
		}
	}
	summary.Children = children
}

// DecisionRequest loads one request with its children, guardians and the
// late invite it was submitted through.
func (d *Decisions) DecisionRequest(ctx context.Context, requestID int64) (*enrollment.DecisionSummary, error) {
	req, err := d.deps.Requests.RequestByID(ctx, requestID, false)
	if err == nil {
		_, err = requestValue(req)
	}
	if err != nil {
		return nil, careplan.ErrBookingRequestNotFound
	}
	summary, err := d.assemble(ctx, req)
	if err != nil {
		return nil, err
	}
	if d.deps.LateInvites == nil || enrollment.NormalizedSubmissionSource(req.SubmissionSource) != enrollmentModels.RequestSourceLateInvite {
		return summary, nil
	}
	invite, err := d.deps.LateInvites.LateInviteByUsedRequestID(ctx, requestID)
	if err != nil {
		if errors.Is(err, enrollment.ErrLateInviteNotFound) {
			return summary, nil
		}
		return nil, fmt.Errorf("decision: load late invite for request %d: %w", requestID, err)
	}
	summary.LateInvite = invite
	return summary, nil
}

func (d *Decisions) assemble(ctx context.Context, req *enrollment.Request) (*enrollment.DecisionSummary, error) {
	phase, err := d.deps.Phases.Phase(ctx, req.PhaseID)
	if err != nil {
		// Phase may have been deleted under us - surface as "phase
		// missing" but don't drop the row from the list.
		d.logger().Warn("decision: phase lookup failed",
			slog.Int64("request_id", req.ID),
			slog.Int64("phase_id", req.PhaseID),
			slog.String("error", err.Error()))
		phase = nil
	}
	children, err := d.deps.Children.ChildrenForRequest(ctx, req.ID, false)
	if err == nil {
		_, err = childValues(children)
	}
	if err != nil {
		return nil, fmt.Errorf("decision: list children for request %d: %w", req.ID, err)
	}
	var guardians []*enrollment.RequestGuardian
	if d.deps.Guardians != nil {
		guardians, err = d.deps.Guardians.RequestGuardians(ctx, []int64{req.ID})
		if err != nil {
			return nil, fmt.Errorf("decision: list guardians for request %d: %w", req.ID, err)
		}
	}
	return &enrollment.DecisionSummary{Request: req, Phase: phase, Children: children, Guardians: guardians}, nil
}
