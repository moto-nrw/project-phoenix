package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// schoolBrand returns the school's mail branding, or empty strings when no
// Enrollment notifications are bound.
func schoolBrand(ctx context.Context, notifications enrollment.Notifications, tenantID int64, baseURL string) (enrollment.MailBrand, error) {
	if notifications == nil {
		return enrollment.MailBrand{}, nil
	}
	return notifications.SchoolBrand(ctx, tenantID, baseURL)
}

// decisionGeneration is one persisted child-state generation of a request as
// the decision flow holds it: the request and its children.
type decisionGeneration struct {
	Request           *enrollment.Request
	Children          []*enrollment.RequestChild
	Phase             *enrollment.Phase
	ImmediateChildIDs map[int64]struct{}
	ParentsURL        string
}

// notifyDecisions routes a decision generation through the Enrollment
// notifications and records the notification mode they pinned on the
// request.
func notifyDecisions(ctx context.Context, notifications enrollment.Notifications, notice decisionGeneration) error {
	if notifications == nil {
		return fmt.Errorf("decision: parent notifications are not configured")
	}
	if notice.Request == nil {
		return fmt.Errorf("decision: request is required for notification mode")
	}
	request := notice.Request
	mode, err := notifications.NotifyDecisions(ctx, enrollment.DecisionNotice{
		Request: enrollment.DecisionRequest{
			ID: request.ID, TenantID: request.TenantID,
			GuardianFirstName: request.GuardianFirstName, GuardianLastName: request.GuardianLastName,
			GuardianEmail: request.GuardianEmail, StatusToken: request.StatusToken,
			NotificationMode: request.DecisionNotificationMode,
		},
		Children: noticeChildren(notice.Children), Phase: notice.Phase,
		ImmediateChildIDs: notice.ImmediateChildIDs, ParentsURL: notice.ParentsURL,
	})
	if err != nil {
		return err
	}
	if request.DecisionNotificationMode == nil {
		request.DecisionNotificationMode = &mode
	}
	return nil
}

func noticeChildren(values []*enrollment.RequestChild) []enrollment.DecisionChild {
	children := make([]enrollment.DecisionChild, 0, len(values))
	for _, child := range values {
		if child == nil {
			// A missing child keeps a digest unresolved and is never mailed.
			children = append(children, enrollment.DecisionChild{})
			continue
		}
		children = append(children, enrollment.DecisionChild{
			ID: child.ID, FirstName: child.FirstName, LastName: child.LastName,
			Status: child.Status, StatusReason: child.StatusReason, ReviewedAt: child.ReviewedAt,
		})
	}
	return children
}
