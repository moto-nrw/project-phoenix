package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// offeringChangeMaxNoteLen bounds free text (in runes) from either side so a
// direct API client cannot persist an unbounded payload.
const offeringChangeMaxNoteLen = 2000

// German pill text of a new request. The staff portal renders it directly;
// the parents portal localizes from the structured event fields.
const offeringChangeCreatedBody = "Anfrage: Betreuungsangebote ändern"

// offeringChangeProposal is the validated result of a guardian's proposal,
// shared by the create and the edit path so an edit can never store what
// the create path would have refused.
type offeringChangeProposal struct {
	note               string
	requestChildID     int64
	selections         []careplan.OfferingChangeSelection
	completeWithdrawal bool
}

// SubmitOfferingChange stores a pending request after validating it exactly as an
// approval would, so a request that cannot be applied is never accepted.
func (s *OfferingChanges) SubmitOfferingChange(ctx context.Context, input careplan.CreateOfferingChangeInput) (*careplan.OfferingChangeRequest, error) {
	proposal, err := s.validateOfferingChangeProposal(ctx, input)
	if err != nil {
		return nil, err
	}
	if err := s.assertNoPendingOfferingChange(ctx, input.StudentID); err != nil {
		return nil, err
	}
	return s.createOfferingChangeRow(ctx, input, proposal)
}

// assertNoPendingOfferingChange keeps one open switch per child.
func (s *OfferingChanges) assertNoPendingOfferingChange(ctx context.Context, studentID int64) error {
	existing, err := s.deps.Rows.PendingForStudent(ctx, studentID)
	if err != nil {
		return fmt.Errorf("offering change: check pending: %w", err)
	}
	if existing != nil {
		return careplan.ErrOfferingChangeAlreadyPending
	}
	return nil
}

func (s *OfferingChanges) validateOfferingChangeProposal(ctx context.Context, input careplan.CreateOfferingChangeInput) (*offeringChangeProposal, error) {
	note, err := s.validateProposalInput(ctx, input)
	if err != nil {
		return nil, err
	}
	scope, err := s.carePeriodAt(ctx, input.StudentID, input.EffectiveFrom)
	if err != nil {
		return nil, err
	}
	if input.EffectiveFrom.Before(scope.phase.ServiceStart) {
		return nil, fmt.Errorf("%w: effective date is before the care period starts", careplan.ErrOfferingChangeInvalid)
	}
	if input.EffectiveFrom.After(scope.phase.ServiceEnd) {
		return nil, fmt.Errorf("%w: effective date is after the care period ends", careplan.ErrOfferingChangeInvalid)
	}
	proposal, err := s.validateProposedSelections(ctx, input, scope)
	if err != nil {
		return nil, err
	}
	proposal.note = note
	return proposal, nil
}

// validateProposalInput checks the gate, the identities, the note and the
// notice period, and returns the trimmed note.
func (s *OfferingChanges) validateProposalInput(ctx context.Context, input careplan.CreateOfferingChangeInput) (string, error) {
	if err := s.changesEnabled(ctx); err != nil {
		return "", err
	}
	if input.StudentID <= 0 || input.AccountID <= 0 {
		return "", fmt.Errorf("%w: student and account are required", careplan.ErrOfferingChangeInvalid)
	}
	note := strings.TrimSpace(input.Note)
	if utf8.RuneCountInString(note) > offeringChangeMaxNoteLen {
		return "", fmt.Errorf("%w: note is too long", careplan.ErrOfferingChangeInvalid)
	}
	earliest, err := s.EarliestEffectiveFrom(ctx)
	if err != nil {
		return "", err
	}
	if input.EffectiveFrom.Before(earliest) {
		return "", fmt.Errorf("%w: effective date is earlier than the school's notice period", careplan.ErrOfferingChangeInvalid)
	}
	return note, nil
}

func (s *OfferingChanges) validateProposedSelections(
	ctx context.Context,
	input careplan.CreateOfferingChangeInput,
	scope *carePeriodScope,
) (*offeringChangeProposal, error) {
	requestChildID := scope.period.RequestChildID
	current, err := s.currentSelections(ctx, requestChildID, input.EffectiveFrom)
	if err != nil {
		return nil, fmt.Errorf("offering change: list current offerings: %w", err)
	}
	authoritative, err := s.bookingsAuthoritative(ctx)
	if err != nil {
		return nil, err
	}
	phaseOfferings, err := s.phaseOfferings(ctx, scope.phase.ID)
	if err != nil {
		return nil, fmt.Errorf("offering change: list phase offerings: %w", err)
	}
	phaseCatalog := offeringsByID(phaseOfferings)
	in := selectionMaterialization{
		phase: scope.phase, requestChildID: requestChildID, effectiveFrom: input.EffectiveFrom,
		selections:              withoutAutomaticSelections(current, input.Selections),
		allowCompleteWithdrawal: authoritative && bookingsHaveCareDays(current, phaseCatalog),
	}
	selections, err := s.validateSelections(ctx, in)
	if err != nil {
		return nil, err
	}
	in.selections = selections
	materialized, err := s.materializedSelections(ctx, in)
	if err != nil {
		return nil, err
	}
	if sameMaterializedOfferingSelections(current, materialized) {
		return nil, fmt.Errorf("%w: offerings are unchanged", careplan.ErrOfferingChangeInvalid)
	}
	completeWithdrawal := in.allowCompleteWithdrawal && !selectionsHaveCareDays(materialized, phaseCatalog)
	if completeWithdrawal && !input.CompleteWithdrawalConfirmed {
		return nil, careplan.ErrCompleteWithdrawalConfirmationRequired
	}
	return &offeringChangeProposal{requestChildID: requestChildID, selections: selections, completeWithdrawal: completeWithdrawal}, nil
}

func (s *OfferingChanges) createOfferingChangeRow(
	ctx context.Context,
	input careplan.CreateOfferingChangeInput,
	proposal *offeringChangeProposal,
) (*careplan.OfferingChangeRequest, error) {
	row, err := newOfferingChangeRow(input, proposal)
	if err != nil {
		return nil, fmt.Errorf("offering change: create: %w", err)
	}
	created, err := s.deps.Rows.Create(ctx, row)
	if err != nil {
		if isPendingOfferingChangeConflict(err) {
			return nil, careplan.ErrOfferingChangeAlreadyPending
		}
		return nil, fmt.Errorf("offering change: create: %w", err)
	}
	if err := s.recordOfferingRequestEvent(ctx, created, careplan.ParentRequestEventSubmitted, input.AccountID,
		map[string]any{"effective_from": offeringChangeEffectiveFrom(created).String()}); err != nil {
		return nil, err
	}
	if err := s.emitPillAfterCommit(ctx, created, requestCreatedEvent(input.AccountID)); err != nil {
		return nil, err
	}
	s.logger().Info("offering change request created",
		slog.Int64("student_id", input.StudentID),
		slog.Int64("request_child_id", proposal.requestChildID),
		slog.String("effective_from", input.EffectiveFrom.String()),
		slog.Int("offering_count", len(proposal.selections)),
	)
	return &created, nil
}

func newOfferingChangeRow(input careplan.CreateOfferingChangeInput, proposal *offeringChangeProposal) (careplan.OfferingChangeRequest, error) {
	payload, err := payloadFromSelections(proposal.selections)
	if err != nil {
		return careplan.OfferingChangeRequest{}, err
	}
	row := careplan.OfferingChangeRequest{
		StudentID: input.StudentID, RequestChildID: proposal.requestChildID, SubmittedBy: input.AccountID,
		Payload: payload, EffectiveFrom: input.EffectiveFrom.String(), Status: careplan.OfferingChangePending,
		CompleteWithdrawalConfirmed: proposal.completeWithdrawal, DecisionSnapshot: json.RawMessage("null"),
	}
	if proposal.completeWithdrawal {
		now := time.Now()
		row.WithdrawalConfirmedBy = &input.AccountID
		row.WithdrawalConfirmedAt = &now
	}
	if proposal.note != "" {
		note := proposal.note
		row.ParentNote = &note
	}
	return row, nil
}

// Edit rewrites the submitter's own pending request (#2267, story 37). It
// replaces the withdraw flow, so the request keeps its id, its share and its
// history. A request that is not the caller's own is reported as missing,
// never as forbidden.
func (s *OfferingChanges) Edit(
	ctx context.Context,
	requestID int64,
	input careplan.CreateOfferingChangeInput,
	expectedVersion string,
) (*careplan.OfferingChangeRequest, error) {
	row, err := s.deps.Rows.FindForUpdate(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if row.StudentID != input.StudentID || row.SubmittedBy != input.AccountID {
		return nil, careplan.ErrOfferingChangeNotFound
	}
	if offeringChangeTerminal(row) {
		return nil, careplan.ErrOfferingChangeNotPending
	}
	if expectedVersion != "" && careplan.ParentRequestVersion(row.UpdatedAt) != expectedVersion {
		return nil, careplan.ErrParentRequestStale
	}
	proposal, err := s.validateOfferingChangeProposal(ctx, input)
	if err != nil {
		return nil, err
	}
	payload, err := payloadFromSelections(proposal.selections)
	if err != nil {
		return nil, err
	}
	if err := s.deps.Rows.UpdatePending(ctx, row.ID, payload, input.EffectiveFrom, optionalNote(proposal.note)); err != nil {
		return nil, err
	}
	edited, err := s.deps.Rows.Find(ctx, row.ID)
	if err != nil {
		return nil, fmt.Errorf("offering change: reload edited request: %w", err)
	}
	if err := s.recordOfferingRequestEvent(ctx, edited, careplan.ParentRequestEventGuardianEdit, input.AccountID,
		map[string]any{"effective_from": offeringChangeEffectiveFrom(edited).String()}); err != nil {
		return nil, err
	}
	return &edited, nil
}

func optionalNote(note string) *string {
	if note == "" {
		return nil
	}
	return &note
}
