package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
)

// MasterDataDecisionDependencies wires the Stammdaten decision (#3354).
// Requests, People, Records, Scope, Hooks and Today are required; the effect
// ports may be nil and the decision then skips that effect.
type MasterDataDecisionDependencies struct {
	Requests    ports.MasterDataRequestRows
	People      ports.MasterDataFieldReviews
	Records     ports.MasterDataRecords
	Scope       ports.ReviewScopeResolver
	Hooks       ports.TransactionHooks
	Today       func() careplan.Date
	Logger      *slog.Logger
	Messenger   ports.RequestMessenger
	Broadcaster ports.MasterDataBroadcaster
	Ledger      ports.RequestLedger
	Shares      ports.ShareVisibility
}

// MasterDataDecisions is the staff side of a parent Stammdaten request: the
// decision, the correction, and the request's part in the cross-kind bulk and
// conflict commands. Care Plan owns the request and the decision; the child's
// record it writes stays with People Directory behind MasterDataRecords.
// Every method runs inside the ambient tenant transaction the caller opened.
type MasterDataDecisions struct {
	requests    ports.MasterDataRequestRows
	people      ports.MasterDataFieldReviews
	records     ports.MasterDataRecords
	scope       ports.ReviewScopeResolver
	hooks       ports.TransactionHooks
	today       func() careplan.Date
	logger      *slog.Logger
	messenger   ports.RequestMessenger
	broadcaster ports.MasterDataBroadcaster
	ledger      ports.RequestLedger
	shares      ports.ShareVisibility
}

var _ masterdatarequests.Decisions = (*MasterDataDecisions)(nil)

func NewMasterDataDecisions(deps MasterDataDecisionDependencies) (*MasterDataDecisions, error) {
	if deps.Requests == nil || deps.People == nil || deps.Records == nil || deps.Scope == nil || deps.Hooks == nil || deps.Today == nil {
		return nil, errors.New("care plan master data decisions: requests, people, records, review scope, transaction hooks, and clock are required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &MasterDataDecisions{
		requests: deps.Requests, people: deps.People, records: deps.Records, scope: deps.Scope,
		hooks: deps.Hooks, today: deps.Today, logger: deps.Logger, messenger: deps.Messenger,
		broadcaster: deps.Broadcaster, ledger: deps.Ledger, shares: deps.Shares,
	}, nil
}

// Decide approves (and applies) or rejects one pending request and returns
// the refreshed row with the child's name.
func (s *MasterDataDecisions) Decide(ctx context.Context, input masterdatarequests.DecideInput) (*careplan.MasterDataReviewItem, error) {
	req, err := s.loadPendingDecisionRequest(ctx, input)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeDecision(ctx, req.StudentID); err != nil {
		return nil, err
	}
	// An approval needs a reason only while the school's policy asks staff for
	// one; a rejection's reason is required by the route either way (#2267).
	if input.Approve && input.ReasonRequired && strings.TrimSpace(input.Reason) == "" {
		return nil, parentrequests.ErrReasonRequired
	}
	var reason *string
	if input.Reason != "" {
		reason = &input.Reason
	}
	var item *careplan.MasterDataReviewItem
	if input.Approve {
		item, err = s.approve(ctx, req, input, reason)
	} else {
		item, err = s.reject(ctx, req, input, reason)
	}
	if err != nil {
		return nil, err
	}
	// The ledger entry is written inside the decision's own transaction, so a
	// rolled-back decision can never leave a "decided" event behind.
	if err := s.record(ctx, ports.RequestLedgerEntry{
		StudentID: item.Request.StudentID, RequestType: masterdatarequests.ParentRequestType,
		RequestID: item.Request.ID, EventType: careplan.ParentRequestEventDecided,
		ActorAccountID: input.ReviewedBy, UpdatedAt: item.Request.UpdatedAt,
		Payload: map[string]any{"approve": input.Approve, "reason": strings.TrimSpace(input.Reason)},
	}); err != nil {
		return nil, fmt.Errorf("review: record decision event: %w", err)
	}
	return item, nil
}

func (s *MasterDataDecisions) loadPendingDecisionRequest(ctx context.Context, input masterdatarequests.DecideInput) (*careplan.StudentDataChangeRequest, error) {
	if input.RequestID <= 0 {
		return nil, masterdatarequests.ErrReviewNotFound
	}
	req, err := s.lockPendingRequest(ctx, input.RequestID)
	if err != nil {
		if errors.Is(err, masterdatarequests.ErrReviewNotFound) || errors.Is(err, masterdatarequests.ErrReviewNotPending) {
			return nil, err
		}
		return nil, fmt.Errorf("review: find pending request: %w", err)
	}
	if input.ExpectedVersion != "" && careplan.ParentRequestVersion(req.UpdatedAt) != input.ExpectedVersion {
		return nil, parentrequests.ErrStale
	}
	return req, nil
}

// lockPendingRequest takes the request row FOR UPDATE and refuses it unless
// it is still open.
func (s *MasterDataDecisions) lockPendingRequest(ctx context.Context, requestID int64) (*careplan.StudentDataChangeRequest, error) {
	row, err := s.requests.FindPendingStudentDataRequest(ctx, requestID)
	switch {
	case errors.Is(err, careplan.ErrStudentDataRequestNotFound):
		return nil, masterdatarequests.ErrReviewNotFound
	case errors.Is(err, careplan.ErrStudentDataRequestNotPending):
		return nil, masterdatarequests.ErrReviewNotPending
	case err != nil:
		return nil, err
	}
	return &row, nil
}

// authorizeDecision gates approve and reject alike: the caller may decide a
// request only for a child inside their review scope.
//
// The child's row is taken FOR UPDATE so the alumnus gate decides on a state
// a concurrent grade transition cannot change underneath it: the transition
// locks exactly this row before flipping it to alumnus, so an unlocked read
// could let an approval through and have the graduation commit before the
// person or student write lands. It is also the transaction's first row
// lock: the apply only re-acquires this row or takes the child's person row
// after it, so no lock order is inverted (#405 review).
func (s *MasterDataDecisions) authorizeDecision(ctx context.Context, studentID int64) error {
	student, err := s.records.LockStudent(ctx, studentID)
	if err != nil {
		return fmt.Errorf("review: load student for decision: %w", err)
	}
	// A graduate or a child whose care ended after filing the request gets the
	// same 404 the rest of the child surface returns (#405 review, #2487).
	if student.Alumnus || student.CareEndedOn(s.today()) {
		return masterdatarequests.ErrReviewNotFound
	}
	scope, err := s.scope(ctx)
	if err != nil {
		return fmt.Errorf("review: resolve reviewer scope: %w", err)
	}
	if !scope.Allows(&student.ReviewStudent) {
		return masterdatarequests.ErrReviewForbidden
	}
	return nil
}

func (s *MasterDataDecisions) reject(
	ctx context.Context, req *careplan.StudentDataChangeRequest, input masterdatarequests.DecideInput, reason *string,
) (*careplan.MasterDataReviewItem, error) {
	if err := s.decideRow(ctx, req.ID, masterdatarequests.StatusRejected, reason, input.ReviewedBy, false); err != nil {
		return nil, decideRowError("reject", err)
	}
	s.logger.Info("staff rejected master data change",
		slog.Int64("request_id", req.ID),
		slog.Int64("student_id", req.StudentID),
		slog.Int64("reviewed_by", input.ReviewedBy),
	)
	if err := s.deferDecisionPill(ctx, req, input.ReviewedBy, input.Reason, false); err != nil {
		return nil, err
	}
	return s.reloadReviewItem(ctx, req.ID, "rejected")
}

func (s *MasterDataDecisions) approve(
	ctx context.Context, req *careplan.StudentDataChangeRequest, input masterdatarequests.DecideInput, reason *string,
) (*careplan.MasterDataReviewItem, error) {
	companionsTrimmed, err := s.applyChange(ctx, req, input.ReviewedBy)
	if err != nil {
		return nil, err
	}
	if err := s.decideRow(ctx, req.ID, masterdatarequests.StatusApproved, reason, input.ReviewedBy, true); err != nil {
		return nil, decideRowError("approve", err)
	}
	s.logger.Info("staff approved master data change",
		slog.Int64("request_id", req.ID),
		slog.Int64("student_id", req.StudentID),
		slog.String("target", req.Target),
		slog.String("field", req.FieldKey),
		slog.Int64("reviewed_by", input.ReviewedBy),
	)
	if err := s.deferDecisionPill(ctx, req, input.ReviewedBy, input.Reason, true); err != nil {
		return nil, err
	}
	s.deferStudentUpdated(ctx, req.StudentID)
	// Only when the write actually trimmed a "läuft mit" link: those links are
	// rows on ANOTHER child's card too. A rename, or a departure approval that
	// left every link intact, must not make open companion forms drop a draft.
	if companionsTrimmed {
		s.deferStudentCompanionsChanged(ctx, req.StudentID)
	}
	return s.reloadReviewItem(ctx, req.ID, "approved")
}

// decideRow closes the pending row. A row another reviewer closed first is
// the lost race, reported as not pending rather than wrapped.
func (s *MasterDataDecisions) decideRow(ctx context.Context, requestID int64, status string, reason *string, reviewedBy int64, applied bool) error {
	err := s.requests.DecideStudentDataRequest(ctx, careplan.StudentDataRequestDecision{
		ID: requestID, Status: status, Reason: reason, ReviewedBy: reviewedBy, Applied: applied,
	})
	if errors.Is(err, careplan.ErrStudentDataRequestNotPending) {
		return masterdatarequests.ErrReviewNotPending
	}
	return err
}

// decideRowError keeps the lost race recognisable and labels everything else.
func decideRowError(verdict string, err error) error {
	if errors.Is(err, masterdatarequests.ErrReviewNotPending) {
		return err
	}
	return fmt.Errorf("review: %s: %w", verdict, err)
}

func (s *MasterDataDecisions) reloadReviewItem(ctx context.Context, requestID int64, status string) (*careplan.MasterDataReviewItem, error) {
	row, err := s.requests.FindStudentDataRequest(ctx, requestID, false)
	if err != nil {
		return nil, fmt.Errorf("review: reload %s request: %w", status, err)
	}
	student, err := s.records.FindStudent(ctx, row.StudentID)
	if err != nil {
		return nil, fmt.Errorf("review: load student: %w", err)
	}
	person, err := s.records.FindPerson(ctx, student.PersonID)
	if err != nil {
		return nil, fmt.Errorf("review: load person: %w", err)
	}
	return &careplan.MasterDataReviewItem{Request: &row, FirstName: person.FirstName, LastName: person.LastName}, nil
}

// record appends one ledger entry; an unwired ledger records nothing.
func (s *MasterDataDecisions) record(ctx context.Context, entry ports.RequestLedgerEntry) error {
	if s.ledger == nil {
		return nil
	}
	return s.ledger.Record(ctx, entry)
}
