package application

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/application/emailbranding"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// Decision notification modes of the enrollment.notify_per_decision setting.
const (
	notifyPerDecisionDigest    = "digest"
	notifyPerDecisionImmediate = "immediate"
)

const schoolLoginImageKey = "loginImageUrl"

// Mail is one Enrollment mail for the Delivery outbox: its kind, the
// template payload, the related request and the idempotency key.
type Mail struct {
	Kind              string
	Payload           map[string]any
	RelatedEntityType string
	RelatedEntityID   int64
	IdempotencyKey    string
}

// MailOutbox enqueues an Enrollment mail on the Delivery outbox in the
// caller's transaction.
type MailOutbox interface {
	EnqueueMail(ctx context.Context, mail Mail) error
}

// NotificationDependencies bind the parent mails to their owners.
type NotificationDependencies struct {
	Modes    NotificationModePin
	Settings NotificationSettings
	Outbox   MailOutbox
	Schools  enrollment.SchoolDirectory
	// Fingerprint identifies a decision state for the outbox idempotency
	// keys.
	Fingerprint Fingerprint
}

// Notifications brands the enrollment mails and routes decision
// notifications through a request's pinned notification mode.
type Notifications struct {
	deps NotificationDependencies
}

// NewNotifications binds the parent mails to their owners.
func NewNotifications(deps NotificationDependencies) *Notifications {
	return &Notifications{deps: deps}
}

// SchoolBrand returns the tenant school's name and logo link for a mail, or
// empty strings when the school cannot be read or is deleted.
func (n *Notifications) SchoolBrand(ctx context.Context, tenantID int64, baseURL string) (string, string) {
	if n.deps.Schools == nil || tenantID == 0 {
		return "", ""
	}
	school, err := n.deps.Schools.FindSchool(ctx, tenantID)
	if err != nil || school == nil || school.Deleted {
		return "", ""
	}
	return school.Name, emailbranding.SchoolLogoURL(baseURL, schoolLoginImageURL(school.Settings))
}

func schoolLoginImageURL(rawSettings string) string {
	if strings.TrimSpace(rawSettings) == "" {
		return ""
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(rawSettings), &settings); err != nil {
		return ""
	}
	url, _ := settings[schoolLoginImageKey].(string)
	return strings.TrimSpace(url)
}

func (n *Notifications) resolveDecisionNotificationMode(ctx context.Context) (string, error) {
	mode := notifyPerDecisionDigest
	if n.deps.Settings != nil {
		resolved, err := n.deps.Settings.NotifyPerDecision(ctx)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(resolved) != "" {
			mode = resolved
		}
	}
	if !supportedNotificationMode(mode) {
		return "", fmt.Errorf("unsupported notification mode %q", mode)
	}
	return mode, nil
}

func supportedNotificationMode(mode string) bool {
	return mode == notifyPerDecisionImmediate || mode == notifyPerDecisionDigest
}

// resolveOrPinDecisionNotificationMode returns the immutable notification mode
// for one enrollment request. The setting stays live until the first
// parent-notifiable decision; PinDecisionNotificationMode then atomically
// preserves that choice for every later sibling transition.
func (n *Notifications) resolveOrPinDecisionNotificationMode(ctx context.Context, request enrollment.DecisionRequest) (string, error) {
	if request.ID <= 0 {
		return "", fmt.Errorf("decision: request is required for notification mode")
	}
	if request.NotificationMode != nil {
		mode := strings.TrimSpace(*request.NotificationMode)
		if !supportedNotificationMode(mode) {
			return "", fmt.Errorf("decision: unsupported pinned notification mode %q", mode)
		}
		return mode, nil
	}
	if n.deps.Modes == nil {
		return "", fmt.Errorf("decision: request repository is required to pin notification mode")
	}

	mode, err := n.resolveDecisionNotificationMode(ctx)
	if err != nil {
		return "", err
	}
	pinned, err := n.deps.Modes.PinDecisionNotificationMode(ctx, request.ID, mode)
	if err != nil {
		return "", fmt.Errorf("decision: pin notification mode: %w", err)
	}
	pinned = strings.TrimSpace(pinned)
	if !supportedNotificationMode(pinned) {
		return "", fmt.Errorf("decision: unsupported pinned notification mode %q", pinned)
	}
	return pinned, nil
}

// NotifyDecisions routes one persisted child-state generation through the
// request's pinned notification mode and returns that mode.
// notice.ImmediateChildIDs contains only children whose new status should
// produce a standalone email. An empty set is still useful for a withdrawal
// that completes a digest-mode request.
func (n *Notifications) NotifyDecisions(ctx context.Context, notice enrollment.DecisionNotice) (string, error) {
	mode, err := n.resolveOrPinDecisionNotificationMode(ctx, notice.Request)
	if err != nil {
		return "", err
	}
	if mode == notifyPerDecisionDigest {
		return mode, n.notifyDigest(ctx, notice)
	}
	return mode, n.notifyImmediately(ctx, notice)
}

func (n *Notifications) notifyDigest(ctx context.Context, notice enrollment.DecisionNotice) error {
	if !allChildrenParentResolved(notice.Children) {
		return nil
	}
	if n.deps.Outbox == nil {
		return fmt.Errorf("decision: outbox is required for digest parent notification")
	}
	return n.enqueueDecisionDigest(ctx, notice)
}

func (n *Notifications) notifyImmediately(ctx context.Context, notice enrollment.DecisionNotice) error {
	if len(notice.ImmediateChildIDs) == 0 {
		return nil
	}
	if n.deps.Outbox == nil {
		return fmt.Errorf("decision: outbox is required for immediate parent notification")
	}
	for _, child := range notice.Children {
		if _, ok := notice.ImmediateChildIDs[child.ID]; !ok {
			continue
		}
		kind, ok := decisionMailKind(child.Status)
		if !ok {
			continue
		}
		if err := n.enqueueImmediateDecisionEmail(ctx, notice, child, kind); err != nil {
			return err
		}
	}
	return nil
}

func allChildrenParentResolved(children []enrollment.DecisionChild) bool {
	if len(children) == 0 {
		return false
	}
	for _, child := range children {
		if !enrollment.ParentResolvedStatus(child.Status) {
			return false
		}
	}
	return true
}

// decisionMailKind is the standalone mail of a parent-visible decision.
func decisionMailKind(status string) (string, bool) {
	switch status {
	case enrollment.ChildStatusApproved:
		return enrollment.MailKindApproved, true
	case enrollment.ChildStatusWaitlisted:
		return enrollment.MailKindWaitlisted, true
	case enrollment.ChildStatusRejected:
		return enrollment.MailKindRejected, true
	default:
		return "", false
	}
}

// decisionDigestIdempotencyKey identifies the exact material decision state.
// The child IDs make sibling reordering irrelevant, while including every
// status and its persisted review generation lets later supported transitions
// enqueue a fresh digest, even when a reopened child returns to an earlier
// status. Retrying the same committed state still deduplicates.
func (n *Notifications) decisionDigestIdempotencyKey(requestID int64, children []enrollment.DecisionChild) string {
	vector := make([]string, 0, len(children))
	for _, child := range children {
		vector = append(vector, decisionChildStateVector(child))
	}
	sort.Strings(vector)
	return fmt.Sprintf("enrollment-decision-digest:%d:%s", requestID, n.deps.Fingerprint([]byte(strings.Join(vector, "|"))))
}

func (n *Notifications) decisionEmailIdempotencyKey(requestID int64, child enrollment.DecisionChild) string {
	state := decisionChildStateVector(child)
	return fmt.Sprintf("enrollment-decision:%d:%d:%s", requestID, child.ID, n.deps.Fingerprint([]byte(state)))
}

func decisionChildStateVector(child enrollment.DecisionChild) string {
	reviewedAt := "unreviewed"
	if child.ReviewedAt != nil {
		reviewedAt = child.ReviewedAt.UTC().Format(time.RFC3339Nano)
	}
	return fmt.Sprintf("%d=%s@%s", child.ID, child.Status, reviewedAt)
}

// decisionMailPayload is the payload every decision mail shares.
func (n *Notifications) decisionMailPayload(ctx context.Context, notice enrollment.DecisionNotice) map[string]any {
	request := notice.Request
	schoolName, logoURL := n.SchoolBrand(ctx, request.TenantID, notice.ParentsURL)
	phaseName := ""
	if notice.Phase != nil {
		phaseName = notice.Phase.Name
	}
	return map[string]any{
		enrollment.EnrollmentPayloadGuardianFirstName: request.GuardianFirstName,
		enrollment.EnrollmentPayloadGuardianLastName:  request.GuardianLastName,
		enrollment.EnrollmentPayloadGuardianEmail:     request.GuardianEmail,
		enrollment.EnrollmentPayloadRecipientEmail:    request.GuardianEmail,
		enrollment.EnrollmentPayloadSchoolName:        schoolName,
		enrollment.EnrollmentPayloadStatusURL:         enrollment.StatusURL(notice.ParentsURL, request.StatusToken),
		enrollment.EnrollmentPayloadLogoURL:           logoURL,
		enrollment.EnrollmentPayloadMotoLogoURL:       emailbranding.MotoLogoURL(notice.ParentsURL),
		enrollment.EnrollmentPayloadPhaseName:         phaseName,
	}
}

func (n *Notifications) enqueueDecisionDigest(ctx context.Context, notice enrollment.DecisionNotice) error {
	names := map[string][]string{
		enrollment.ChildStatusApproved:   {},
		enrollment.ChildStatusWaitlisted: {},
		enrollment.ChildStatusRejected:   {},
		enrollment.ChildStatusWithdrawn:  {},
	}
	for _, child := range notice.Children {
		if list, ok := names[child.Status]; ok {
			names[child.Status] = append(list, strings.TrimSpace(child.FirstName+" "+child.LastName))
		}
	}
	payload := n.decisionMailPayload(ctx, notice)
	payload["approved_names"] = names[enrollment.ChildStatusApproved]
	payload["waitlisted_names"] = names[enrollment.ChildStatusWaitlisted]
	payload["rejected_names"] = names[enrollment.ChildStatusRejected]
	payload["withdrawn_names"] = names[enrollment.ChildStatusWithdrawn]
	if err := n.deps.Outbox.EnqueueMail(ctx, Mail{
		Kind:              enrollment.MailKindDecisionDigest,
		Payload:           payload,
		RelatedEntityType: enrollment.MailRelatedRequest,
		RelatedEntityID:   notice.Request.ID,
		IdempotencyKey:    n.decisionDigestIdempotencyKey(notice.Request.ID, notice.Children),
	}); err != nil {
		return fmt.Errorf("decision: enqueue parent decision digest: %w", err)
	}
	return nil
}

func (n *Notifications) enqueueImmediateDecisionEmail(ctx context.Context, notice enrollment.DecisionNotice, child enrollment.DecisionChild, kind string) error {
	payload := n.decisionMailPayload(ctx, notice)
	payload[enrollment.EnrollmentPayloadChildNames] = []string{child.FirstName + " " + child.LastName}
	if notice.Phase != nil && notice.Phase.ShowStatusReasonToParent && child.StatusReason != nil && *child.StatusReason != "" {
		payload[enrollment.EnrollmentPayloadStatusReason] = *child.StatusReason
	}

	if err := n.deps.Outbox.EnqueueMail(ctx, Mail{
		Kind:              kind,
		Payload:           payload,
		RelatedEntityType: enrollment.MailRelatedRequest,
		RelatedEntityID:   notice.Request.ID,
		IdempotencyKey:    n.decisionEmailIdempotencyKey(notice.Request.ID, child),
	}); err != nil {
		return fmt.Errorf("decision: enqueue parent decision email: %w", err)
	}
	return nil
}
