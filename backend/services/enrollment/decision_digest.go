package enrollment

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"time"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
)

type decisionNotificationSettings interface {
	ResolveString(ctx context.Context, key string) (string, error)
}

type decisionNotificationRequestRepository interface {
	PinDecisionNotificationMode(ctx context.Context, requestID int64, proposedMode string) (string, error)
}

type decisionNotificationDependencies struct {
	requests   decisionNotificationRequestRepository
	settings   decisionNotificationSettings
	outbox     platformModels.OutboxEnqueuer
	schools    platformModels.SchoolRepository
	parentsURL string
}

func resolveDecisionNotificationMode(ctx context.Context, settings decisionNotificationSettings) (string, error) {
	mode := configModel.EnrollmentNotifyPerDecisionDigest
	if settings != nil {
		resolved, err := settings.ResolveString(ctx, configModel.KeyEnrollmentNotifyPerDecision)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(resolved) != "" {
			mode = resolved
		}
	}
	if mode != configModel.EnrollmentNotifyPerDecisionImmediate && mode != configModel.EnrollmentNotifyPerDecisionDigest {
		return "", fmt.Errorf("unsupported notification mode %q", mode)
	}
	return mode, nil
}

// resolveOrPinDecisionNotificationMode returns the immutable notification mode
// for one enrollment request. The setting stays live until the first
// parent-notifiable decision; PinDecisionNotificationMode then atomically
// preserves that choice for every later sibling transition.
func resolveOrPinDecisionNotificationMode(
	ctx context.Context,
	deps decisionNotificationDependencies,
	request *enrollmentModels.Request,
) (string, error) {
	if request == nil || request.ID <= 0 {
		return "", fmt.Errorf("decision: request is required for notification mode")
	}
	if request.DecisionNotificationMode != nil {
		mode := strings.TrimSpace(*request.DecisionNotificationMode)
		if mode != configModel.EnrollmentNotifyPerDecisionImmediate && mode != configModel.EnrollmentNotifyPerDecisionDigest {
			return "", fmt.Errorf("decision: unsupported pinned notification mode %q", mode)
		}
		return mode, nil
	}
	if deps.requests == nil {
		return "", fmt.Errorf("decision: request repository is required to pin notification mode")
	}

	mode, err := resolveDecisionNotificationMode(ctx, deps.settings)
	if err != nil {
		return "", err
	}
	pinned, err := deps.requests.PinDecisionNotificationMode(ctx, request.ID, mode)
	if err != nil {
		return "", fmt.Errorf("decision: pin notification mode: %w", err)
	}
	pinned = strings.TrimSpace(pinned)
	if pinned != configModel.EnrollmentNotifyPerDecisionImmediate && pinned != configModel.EnrollmentNotifyPerDecisionDigest {
		return "", fmt.Errorf("decision: unsupported pinned notification mode %q", pinned)
	}
	request.DecisionNotificationMode = &pinned
	return pinned, nil
}

// enqueueDecisionNotifications routes one persisted child-state generation
// through the request's pinned notification mode. immediateChildIDs contains
// only children whose new status should produce a standalone email. An empty
// set is still useful for a withdrawal that completes a digest-mode request.
func enqueueDecisionNotifications(
	ctx context.Context,
	deps decisionNotificationDependencies,
	request *enrollmentModels.Request,
	children []*enrollmentModels.RequestChild,
	phase *enrollmentModels.Phase,
	immediateChildIDs map[int64]struct{},
) error {
	mode, err := resolveOrPinDecisionNotificationMode(ctx, deps, request)
	if err != nil {
		return err
	}

	if mode == configModel.EnrollmentNotifyPerDecisionDigest {
		if !allChildrenParentResolved(children) {
			return nil
		}
		if deps.outbox == nil {
			return fmt.Errorf("decision: outbox is required for digest parent notification")
		}
		return enqueueDecisionDigest(ctx, deps.outbox, deps.schools, deps.parentsURL, request, children, phase)
	}

	if len(immediateChildIDs) == 0 {
		return nil
	}
	if deps.outbox == nil {
		return fmt.Errorf("decision: outbox is required for immediate parent notification")
	}
	for _, child := range children {
		if child == nil {
			continue
		}
		if _, ok := immediateChildIDs[child.ID]; !ok {
			continue
		}
		status := DecisionStatus(child.Status)
		if !isParentVisibleDecision(status) {
			continue
		}
		if err := enqueueImmediateDecisionEmail(ctx, deps.outbox, deps.schools, deps.parentsURL, request, child, phase, status, child.StatusReason); err != nil {
			return err
		}
	}
	return nil
}

func allChildrenParentResolved(children []*enrollmentModels.RequestChild) bool {
	if len(children) == 0 {
		return false
	}
	for _, child := range children {
		if child == nil {
			return false
		}
		switch child.Status {
		case enrollmentModels.ChildStatusApproved, enrollmentModels.ChildStatusRejected,
			enrollmentModels.ChildStatusWaitlisted, enrollmentModels.ChildStatusWithdrawn:
		default:
			return false
		}
	}
	return true
}

// decisionDigestIdempotencyKey identifies the exact material decision state.
// The child IDs make sibling reordering irrelevant, while including every
// status and its persisted review generation lets later supported transitions
// enqueue a fresh digest, even when a reopened child returns to an earlier
// status. Retrying the same committed state still deduplicates.
func decisionDigestIdempotencyKey(requestID int64, children []*enrollmentModels.RequestChild) string {
	vector := make([]string, 0, len(children))
	for _, child := range children {
		if child == nil {
			continue
		}
		vector = append(vector, decisionChildStateVector(child))
	}
	sort.Strings(vector)
	sum := sha256.Sum256([]byte(strings.Join(vector, "|")))
	return fmt.Sprintf("enrollment-decision-digest:%d:%x", requestID, sum)
}

func decisionEmailIdempotencyKey(requestID int64, child *enrollmentModels.RequestChild) string {
	state := decisionChildStateVector(child)
	sum := sha256.Sum256([]byte(state))
	return fmt.Sprintf("enrollment-decision:%d:%d:%x", requestID, child.ID, sum)
}

func decisionChildStateVector(child *enrollmentModels.RequestChild) string {
	reviewedAt := "unreviewed"
	if child.ReviewedAt != nil {
		reviewedAt = child.ReviewedAt.UTC().Format(time.RFC3339Nano)
	}
	return fmt.Sprintf("%d=%s@%s", child.ID, child.Status, reviewedAt)
}

func enqueueDecisionDigest(
	ctx context.Context,
	outbox platformModels.OutboxEnqueuer,
	schoolRepo platformModels.SchoolRepository,
	parentsURL string,
	request *enrollmentModels.Request,
	children []*enrollmentModels.RequestChild,
	phase *enrollmentModels.Phase,
) error {
	if outbox == nil {
		return fmt.Errorf("decision: outbox is required for digest parent notification")
	}
	approved := []string{}
	waitlisted := []string{}
	rejected := []string{}
	withdrawn := []string{}
	for _, child := range children {
		if child == nil {
			continue
		}
		name := strings.TrimSpace(child.FirstName + " " + child.LastName)
		switch child.Status {
		case enrollmentModels.ChildStatusApproved:
			approved = append(approved, name)
		case enrollmentModels.ChildStatusWaitlisted:
			waitlisted = append(waitlisted, name)
		case enrollmentModels.ChildStatusRejected:
			rejected = append(rejected, name)
		case enrollmentModels.ChildStatusWithdrawn:
			withdrawn = append(withdrawn, name)
		}
	}
	schoolName, logoURL := emailBrandForSchool(ctx, schoolRepo, request.TenantID, parentsURL)
	phaseName := ""
	if phase != nil {
		phaseName = phase.Name
	}
	payload := map[string]any{
		EnrollmentPayloadGuardianFirstName: request.GuardianFirstName,
		EnrollmentPayloadGuardianLastName:  request.GuardianLastName,
		EnrollmentPayloadGuardianEmail:     request.GuardianEmail,
		EnrollmentPayloadRecipientEmail:    request.GuardianEmail,
		EnrollmentPayloadSchoolName:        schoolName,
		EnrollmentPayloadStatusURL:         enrollmentStatusURL(parentsURL, request.StatusToken),
		EnrollmentPayloadLogoURL:           logoURL,
		EnrollmentPayloadMotoLogoURL:       motoLogoURL(parentsURL),
		EnrollmentPayloadPhaseName:         phaseName,
		"approved_names":                   approved,
		"waitlisted_names":                 waitlisted,
		"rejected_names":                   rejected,
		"withdrawn_names":                  withdrawn,
	}
	if err := outbox.EnqueueOutbox(ctx, platformModels.OutboxEnqueueRequest{
		Kind:              platformModels.EmailKindEnrollmentDecisionDigest,
		Payload:           payload,
		RelatedEntityType: platformModels.EmailRelatedTypeEnrollmentRequest,
		RelatedEntityID:   request.ID,
		IdempotencyKey:    decisionDigestIdempotencyKey(request.ID, children),
	}); err != nil {
		return fmt.Errorf("decision: enqueue parent decision digest: %w", err)
	}
	return nil
}

func enqueueImmediateDecisionEmail(
	ctx context.Context,
	outbox platformModels.OutboxEnqueuer,
	schoolRepo platformModels.SchoolRepository,
	parentsURL string,
	request *enrollmentModels.Request,
	child *enrollmentModels.RequestChild,
	phase *enrollmentModels.Phase,
	status DecisionStatus,
	reason *string,
) error {
	if outbox == nil {
		return fmt.Errorf("decision: outbox is required for immediate parent notification")
	}

	var kind string
	switch status {
	case DecisionApproved:
		kind = platformModels.EmailKindEnrollmentApproved
	case DecisionWaitlisted:
		kind = platformModels.EmailKindEnrollmentWaitlisted
	case DecisionRejected:
		kind = platformModels.EmailKindEnrollmentRejected
	default:
		return nil
	}

	schoolName, logoURL := emailBrandForSchool(ctx, schoolRepo, request.TenantID, parentsURL)
	statusURL := enrollmentStatusURL(parentsURL, request.StatusToken)
	phaseName := ""
	if phase != nil {
		phaseName = phase.Name
	}

	payload := map[string]any{
		EnrollmentPayloadGuardianFirstName: request.GuardianFirstName,
		EnrollmentPayloadGuardianLastName:  request.GuardianLastName,
		EnrollmentPayloadGuardianEmail:     request.GuardianEmail,
		EnrollmentPayloadSchoolName:        schoolName,
		EnrollmentPayloadStatusURL:         statusURL,
		EnrollmentPayloadLogoURL:           logoURL,
		EnrollmentPayloadMotoLogoURL:       motoLogoURL(parentsURL),
		EnrollmentPayloadChildNames:        []string{child.FirstName + " " + child.LastName},
		EnrollmentPayloadRecipientEmail:    request.GuardianEmail,
		EnrollmentPayloadPhaseName:         phaseName,
	}
	if phase != nil && phase.ShowStatusReasonToParent && reason != nil && *reason != "" {
		payload["status_reason"] = *reason
	}

	if err := outbox.EnqueueOutbox(ctx, platformModels.OutboxEnqueueRequest{
		Kind:              kind,
		Payload:           payload,
		RelatedEntityType: platformModels.EmailRelatedTypeEnrollmentRequest,
		RelatedEntityID:   request.ID,
		IdempotencyKey:    decisionEmailIdempotencyKey(request.ID, child),
	}); err != nil {
		return fmt.Errorf("decision: enqueue parent decision email: %w", err)
	}
	return nil
}
