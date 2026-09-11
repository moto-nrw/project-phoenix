package users

import (
	"context"
	"log/slog"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Co-guardian notices (#2267, story 47). A staff decision on one parent's
// Stammdaten request changes the child's record for the whole family, and the
// other guardians used to hear nothing about it.
//
// The neutral line names the AREA that changed, never the value: a co-guardian
// who was not made an explicit recipient has no business reading the new phone
// number or the requested name. Whoever the parent did share the request with
// gets the same pill the submitter gets.

// SetRequestShareVisibility wires the sharing resolver after construction, so
// every existing construction site keeps working.
func (s *masterDataReviewService) SetRequestShareVisibility(resolver parentmessaging.ShareVisibilityResolver) {
	if s != nil {
		s.shareVisibility = resolver
	}
}

// sharedRecipients resolves the explicit recipients, tolerating an unwired
// resolver. On error everyone falls back to the neutral line.
func (s *masterDataReviewService) sharedRecipients(
	ctx context.Context, req *userModels.StudentDataChangeRequest,
) []int64 {
	if s.shareVisibility == nil {
		return nil
	}
	accountIDs, err := s.shareVisibility.SharedRecipientAccountIDs(
		ctx, req.StudentID, userModels.ParentRequestTypeMasterData, req.ID,
	)
	if err != nil {
		s.logger.Warn("resolving explicit request recipients failed, falling back to neutral notices",
			slog.Int64("request_id", req.ID),
			slog.Int64("student_id", req.StudentID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	return accountIDs
}

// notifyOtherGuardiansAfterCommit posts the decision to the child's other
// guardians. The audience is resolved inside the transaction (a tenant-scoped
// read); only the pills go out after commit.
func (s *masterDataReviewService) notifyOtherGuardiansAfterCommit(
	ctx context.Context,
	req *userModels.StudentDataChangeRequest,
	ev parentmessaging.ChildEvent,
) {
	if s.emitter == nil {
		return
	}
	audience, err := s.emitter.ResolveDecisionAudience(
		ctx, req.StudentID, req.SubmittedBy, s.sharedRecipients(ctx, req),
	)
	if err != nil {
		s.logger.Warn("co-guardian notice: resolving guardians failed",
			slog.Int64("request_id", req.ID),
			slog.Int64("student_id", req.StudentID),
			slog.String("error", err.Error()),
		)
		return
	}
	if len(audience.Full) == 0 && len(audience.Neutral) == 0 {
		return
	}
	tenantID := tenant.FromContext(ctx)
	neutral := ev
	neutral.Body = "Betreuungsstand geändert: Stammdaten"
	studentID := req.StudentID
	tenant.RegisterAfterCommit(ctx, func() {
		s.emitter.EmitDecisionAudience(tenantID, studentID, audience, ev, neutral)
	})
}

// The factory wires the co-guardian resolver by type assertion, so a service
// that silently stopped satisfying this setter would leave its domain's
// co-guardians hearing nothing, with nothing failing. This makes that a
// compile error instead (#2267, story 47).
var _ interface {
	SetRequestShareVisibility(parentmessaging.ShareVisibilityResolver)
} = (*masterDataReviewService)(nil)
