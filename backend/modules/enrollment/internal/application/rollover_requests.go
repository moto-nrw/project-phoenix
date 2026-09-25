package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/emailbranding"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type rolloverChildAttributes struct {
	targetGradeLevel  *int16
	targetSchoolClass *string
	status            string
	reviewReason      *string
}

func (s *Rollovers) rollSourceRequests(ctx context.Context, input rolloverRequestInput) error {
	requestOrder, childrenByRequest := groupRolloverChildrenByRequest(input.sourceChildren)
	values, err := s.deps.Requests.RequestsByID(ctx, requestOrder)
	if err != nil {
		return fmt.Errorf("rollover: load source requests: %w", err)
	}
	requests, err := requestValues(values)
	if err != nil {
		return fmt.Errorf("rollover: load source requests: %w", err)
	}
	requestsByID := make(map[int64]*enrollmentModels.Request, len(requests))
	for _, request := range requests {
		requestsByID[request.ID] = request
	}
	for _, sourceRequestID := range requestOrder {
		sourceRequest := requestsByID[sourceRequestID]
		if sourceRequest == nil {
			return fmt.Errorf("rollover: source request %d not found", sourceRequestID)
		}
		if err := s.rollOneRequest(ctx, input, sourceRequest, childrenByRequest[sourceRequestID]); err != nil {
			return err
		}
		input.result.RequestCount++
	}
	return nil
}

func groupRolloverChildrenByRequest(sourceChildren []*RequestChild) ([]int64, map[int64][]*RequestChild) {
	childrenByRequest := make(map[int64][]*RequestChild, len(sourceChildren))
	requestOrder := make([]int64, 0)
	for _, child := range sourceChildren {
		if _, seen := childrenByRequest[child.RequestID]; !seen {
			requestOrder = append(requestOrder, child.RequestID)
		}
		childrenByRequest[child.RequestID] = append(childrenByRequest[child.RequestID], child)
	}
	return requestOrder, childrenByRequest
}

// rollOneRequest creates the per-parent renewal envelope (new Request)
// plus per-child rows + their care-offering copies, then enqueues the
// renewal email.
func (s *Rollovers) rollOneRequest(ctx context.Context, input rolloverRequestInput, sourceRequest *enrollmentModels.Request, sourceChildren []*RequestChild) error {
	newRequest, err := s.createRolloverRequest(ctx, input, sourceRequest)
	if err != nil {
		return err
	}
	childNames := make([]string, 0, len(sourceChildren))
	for _, sourceChild := range sourceChildren {
		childName, err := s.rollSourceChild(ctx, input, newRequest, sourceChild)
		if err != nil {
			return err
		}
		childNames = append(childNames, childName)
	}
	s.enqueueRenewalEmail(ctx, input.newPhase, newRequest, childNames, input.result)
	return nil
}

func (s *Rollovers) createRolloverRequest(ctx context.Context, input rolloverRequestInput, sourceRequest *enrollmentModels.Request) (*enrollmentModels.Request, error) {
	statusToken, err := enrollment.NewStatusToken(s.deps.Random)
	if err != nil {
		return nil, fmt.Errorf("rollover: generate status token: %w", err)
	}
	request := &enrollmentModels.Request{
		PhaseID:           input.newPhase.ID,
		SchemaID:          input.newPhase.FormSchemaID,
		GuardianFirstName: sourceRequest.GuardianFirstName,
		GuardianLastName:  sourceRequest.GuardianLastName,
		GuardianEmail:     sourceRequest.GuardianEmail,
		GuardianPhone:     sourceRequest.GuardianPhone,
		GuardianAccountID: sourceRequest.GuardianAccountID,
		ConsentFlags:      sourceRequest.ConsentFlags,
		CustomData:        sourceRequest.CustomData,
		StatusToken:       statusToken,
		SubmittedAt:       time.Now(),
	}
	request.TenantID = input.tenantID
	if err := s.insertRequest(ctx, request); err != nil {
		return nil, fmt.Errorf("rollover: create request: %w", err)
	}
	return request, nil
}

// insertRequest encodes, inserts and decodes a request back into place.
func (s *Rollovers) insertRequest(ctx context.Context, request *enrollmentModels.Request) error {
	value, err := requestInput(request)
	if err != nil {
		return err
	}
	if err := s.deps.Requests.InsertRequest(ctx, value); err != nil {
		return err
	}
	result, err := requestValue(value)
	if err != nil {
		return err
	}
	*request = *result
	return nil
}

func (s *Rollovers) rollSourceChild(ctx context.Context, input rolloverRequestInput, newRequest *enrollmentModels.Request, source *RequestChild) (string, error) {
	attributes := rolloverAttributesForSource(input, source)
	sourceID := source.ID
	child := &RequestChild{
		RequestID:             newRequest.ID,
		FirstName:             source.FirstName,
		LastName:              source.LastName,
		DateOfBirth:           source.DateOfBirth,
		TargetGradeLevel:      attributes.targetGradeLevel,
		TargetSchoolClass:     attributes.targetSchoolClass,
		CustomData:            source.CustomData,
		Status:                attributes.status,
		ActivationMode:        source.ActivationMode,
		SortOrder:             source.SortOrder,
		RolloverSourceChildID: &sourceID,
		ReviewReason:          attributes.reviewReason,
	}
	child.TenantID = input.tenantID
	if err := s.insertChild(ctx, child); err != nil {
		if errors.Is(err, enrollment.ErrRolloverSourceChildTaken) {
			return "", fmt.Errorf("%w: source child %d", enrollment.ErrRolloverSourceAlreadyRolled, source.ID)
		}
		return "", fmt.Errorf("rollover: create request_child: %w", err)
	}
	if err := s.copyRolloverOfferings(ctx, input, source.ID, child.ID); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s %s", source.FirstName, source.LastName), nil
}

// insertChild encodes, inserts and decodes a child back into place.
func (s *Rollovers) insertChild(ctx context.Context, child *RequestChild) error {
	value, err := childInput(child)
	if err != nil {
		return err
	}
	if err := s.deps.Children.InsertChild(ctx, value); err != nil {
		return err
	}
	result, err := childValue(value)
	if err != nil {
		return err
	}
	*child = *result
	return nil
}

func rolloverAttributesForSource(input rolloverRequestInput, source *RequestChild) rolloverChildAttributes {
	grade, reviewReason := classifyRolloverGrade(
		source.TargetGradeLevel,
		input.newPhase.RolloverBumpsGrade,
		input.collectGradeLevel,
		input.maxGrade,
	)
	attributes := rolloverChildAttributes{targetGradeLevel: grade}
	if reviewReason == "" {
		attributes.status = renewalInitialStatus(*input.newPhase.RolloverMode)
		input.result.RolledCount++
	} else {
		attributes.status = enrollmentModels.ChildStatusPendingAdminReview
		attributes.reviewReason = &reviewReason
		input.result.ReviewCount++
		input.result.ReviewByReason[reviewReason]++
	}
	if input.collectGradeLevel && !input.newPhase.RolloverBumpsGrade && reviewReason == "" {
		attributes.targetSchoolClass = source.TargetSchoolClass
	}
	return attributes
}

// copyRolloverOfferings carries the booking effective at the END of the
// source phase into the new child: the offering reference is remapped to
// the target phase's clone, the effective weekday selection (parent-
// selected, manual, and automatically derived days) plus notes seed the new
// submission. Effective bookings use the new phase's service window, never
// the historical source intervals (#2249).
func (s *Rollovers) copyRolloverOfferings(ctx context.Context, input rolloverRequestInput, sourceChildID, newChildID int64) error {
	offerings, err := s.deps.Children.RequestChildOfferingsAtDate(ctx, sourceChildID, input.sourcePhase.ServiceEndDate)
	if err != nil {
		return fmt.Errorf("rollover: list source offerings: %w", err)
	}
	if len(offerings) == 0 {
		return nil
	}
	if s.deps.Bookings == nil {
		return fmt.Errorf("rollover requires Care Plan booking commands")
	}
	choices, bookings, err := rolloverOfferingCopies(input, sourceChildID, offerings)
	if err != nil {
		return err
	}
	if err := s.deps.Children.RecordSubmittedOfferingChoices(ctx, newChildID, choices); err != nil {
		return fmt.Errorf("rollover: record submitted offerings: %w", err)
	}
	if err := s.deps.Bookings.RecordCareBookings(ctx, newChildID, bookings); err != nil {
		return fmt.Errorf("rollover: record effective bookings: %w", err)
	}
	return nil
}

// rolloverOfferingCopies remaps the source child's offerings onto the target
// phase's clones.
func rolloverOfferingCopies(input rolloverRequestInput, sourceChildID int64, offerings []*enrollment.RequestChildOffering) ([]enrollment.SubmittedOfferingChoice, []enrollment.CareBookingInput, error) {
	start := calendar.Date(input.newPhase.ServiceStartDate)
	until := calendar.Date(input.newPhase.ServiceEndDate).AddDays(1)
	choices := make([]enrollment.SubmittedOfferingChoice, 0, len(offerings))
	bookings := make([]enrollment.CareBookingInput, 0, len(offerings))
	for _, offering := range offerings {
		targetOfferingID, ok := input.offeringIDMap[offering.CareOfferingID]
		if !ok {
			return nil, nil, fmt.Errorf(
				"rollover: source care offering %d of child %d has no clone in the target phase",
				offering.CareOfferingID, sourceChildID,
			)
		}
		manual := offering.ManualSelectedDays
		if len(manual) == 0 && len(offering.AutomaticSelectedDays) == 0 {
			manual = offering.SelectedDays
		}
		choices = append(choices, enrollment.SubmittedOfferingChoice{
			CareOfferingID: targetOfferingID, SelectedDays: manual, Notes: offering.Notes,
		})
		bookings = append(bookings, enrollment.CareBookingInput{
			CareOfferingID: targetOfferingID, ManualSelectedDays: manual, AutomaticSelectedDays: offering.AutomaticSelectedDays,
			ValidFrom: &start, ValidUntil: &until,
		})
	}
	return choices, bookings, nil
}

func (s *Rollovers) enqueueRenewalEmail(ctx context.Context, newPhase *enrollment.Phase, req *enrollmentModels.Request, childNames []string, result *enrollment.RolloverResult) {
	if s.deps.Outbox == nil {
		return
	}
	if req.GuardianEmail == "" {
		result.SkippedEmptyEmail++
		return
	}
	kind := enrollment.MailKindRolloverOptOut
	if *newPhase.RolloverMode == enrollment.PhaseRolloverModeOptIn {
		kind = enrollment.MailKindRolloverOptIn
	}
	if err := s.deps.Outbox.EnqueueMail(ctx, Mail{
		Kind:              kind,
		Payload:           s.renewalMailPayload(ctx, newPhase, req, childNames),
		RelatedEntityType: enrollment.MailRelatedRequest,
		RelatedEntityID:   req.ID,
	}); err != nil {
		s.deps.Logger.Warn("rollover: enqueue renewal email failed",
			slog.Int64("request_id", req.ID),
			slog.String("kind", kind),
			slog.String("error", err.Error()),
		)
		return
	}
	result.EnqueuedEmails++
}

func (s *Rollovers) renewalMailPayload(ctx context.Context, newPhase *enrollment.Phase, req *enrollmentModels.Request, childNames []string) map[string]any {
	schoolName, logoURL := schoolBrand(ctx, s.deps.Notifications, req.TenantID, s.deps.ParentsURL)
	deadlineStr := ""
	if newPhase.RolloverDeadline != nil {
		deadlineStr = newPhase.RolloverDeadline.Format("02.01.2006")
	}
	return map[string]any{
		enrollment.EnrollmentPayloadGuardianFirstName: req.GuardianFirstName,
		enrollment.EnrollmentPayloadGuardianLastName:  req.GuardianLastName,
		enrollment.EnrollmentPayloadGuardianEmail:     req.GuardianEmail,
		enrollment.EnrollmentPayloadSchoolName:        schoolName,
		enrollment.EnrollmentPayloadPhaseName:         newPhase.Name,
		enrollment.EnrollmentPayloadStatusURL:         enrollment.StatusURL(s.deps.ParentsURL, req.StatusToken),
		enrollment.EnrollmentPayloadLogoURL:           logoURL,
		enrollment.EnrollmentPayloadMotoLogoURL:       emailbranding.MotoLogoURL(s.deps.ParentsURL),
		enrollment.EnrollmentPayloadChildNames:        childNames,
		enrollment.EnrollmentPayloadRecipientEmail:    req.GuardianEmail,
		enrollment.EnrollmentPayloadRolloverDeadline:  deadlineStr,
	}
}
