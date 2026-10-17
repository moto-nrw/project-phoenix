package notifications

import (
	"context"
	"errors"
	"fmt"
)

// RequestDecisionPreferences is the exact consent operation the producer uses.
type RequestDecisionPreferences interface {
	FilterOptedIn(context.Context, string, []int64) ([]int64, error)
}

// RequestDecisionNotifier produces consent-filtered guardian request decisions.
type RequestDecisionNotifier struct {
	notifier    Service
	preferences RequestDecisionPreferences
}

func NewRequestDecisionNotifier(notifier Service, preferences RequestDecisionPreferences) *RequestDecisionNotifier {
	if notifier == nil || preferences == nil {
		panic("request decision notifier: notifier and preferences are required")
	}
	return &RequestDecisionNotifier{notifier: notifier, preferences: preferences}
}

func (n *RequestDecisionNotifier) NotifyRequestDecision(ctx context.Context, tenantID, studentID, guardianAccountID int64, locale, refTable string, refID int64, requestType, requestStatus string) error {
	optedIn, err := n.preferences.FilterOptedIn(ctx, TypeParentRequestDecided, []int64{guardianAccountID})
	if err != nil {
		return err
	}
	if len(optedIn) == 0 {
		return nil
	}
	title, body := ParentRequestDecisionCopy(locale, requestType, requestStatus)
	err = n.notifier.Notify(ctx, Event{
		Type:           TypeParentRequestDecided,
		IdempotencyKey: fmt.Sprintf("parent-request-decision:%s:%d:%s", refTable, refID, requestStatus),
		RelatedType:    refTable, RelatedID: refID, Title: title, Body: body,
		DeepLink: fmt.Sprintf("/children/%d", studentID), Priority: PriorityNormal,
		Audience: Audience{TenantID: tenantID, Scope: ScopeGuardian, GuardianAccountIDs: optedIn, StudentIDs: []int64{studentID}},
	})
	if errors.Is(err, ErrDisabled) || errors.Is(err, ErrOutsideActiveWindow) {
		return nil
	}
	return err
}
