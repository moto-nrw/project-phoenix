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
		return nil
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
		EnrollmentPayloadStatusURL:         fmt.Sprintf("%s/enroll/status/%s", parentsURL, request.StatusToken),
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
