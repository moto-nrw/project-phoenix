package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	deliveryPorts "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// notifyGroupRecipients tells every reached guardian about a new group
// message, by push and by e-mail, under the same consent as a 1:1 OGS
// message: push only for guardians who opted in, e-mail for everyone who did
// not decline. Both run in the request transaction, so the message and its
// notices commit together. Each guardian gets one notice per message, however
// many of their children it reached.
func (s *Service) notifyGroupRecipients(ctx context.Context, thread *deliveryPorts.ParentGroupThread, messageID int64, recipients []deliveryPorts.ParentGroupRecipient) error {
	children := make(map[int64][]int64)
	accounts := make([]int64, 0)
	for _, recipient := range recipients {
		if _, seen := children[recipient.GuardianAccountID]; !seen {
			accounts = append(accounts, recipient.GuardianAccountID)
		}
		children[recipient.GuardianAccountID] = append(children[recipient.GuardianAccountID], recipient.StudentID)
	}
	s.pushGroupMessage(ctx, thread, messageID, accounts, children)
	return s.mailGroupMessage(ctx, thread, messageID, accounts)
}

func (s *Service) pushGroupMessage(ctx context.Context, thread *deliveryPorts.ParentGroupThread, messageID int64, accounts []int64, children map[int64][]int64) {
	if s.Notifier == nil || s.Preferences == nil {
		return
	}
	optedIn, err := s.Preferences.FilterOptedIn(ctx, deliveryPorts.TypeParentMessage, accounts)
	if err != nil {
		s.Logger.Warn("messaging: filter opted-in group recipients failed",
			slog.Int64("group_thread_id", thread.ID),
			slog.String("error", err.Error()),
		)
		return
	}
	for _, accountID := range optedIn {
		locale := s.guardianLocale(ctx, accountID)
		title, body := s.deliveryPolicy.ParentMessageCopy(locale)
		err := s.Notifier.Notify(ctx, deliveryPorts.NotificationEvent{
			Type:           deliveryPorts.TypeParentMessage,
			IdempotencyKey: fmt.Sprintf("parent-group-message:%d:%d", messageID, accountID),
			RelatedType:    "parent_group_thread", RelatedID: thread.ID,
			Title: title, Body: body,
			DeepLink: "/messages/group/" + strconv.FormatInt(thread.ID, 10),
			Priority: deliveryPorts.PriorityNormal,
			Audience: deliveryPorts.NotificationAudience{
				TenantID:           thread.TenantID,
				Scope:              deliveryPorts.ScopeGuardian,
				GuardianAccountIDs: []int64{accountID},
				StudentIDs:         children[accountID],
			},
		}.Public())
		switch {
		case s.deliveryPolicy.IsNotificationsDisabled(err), s.deliveryPolicy.IsOutsideActiveWindow(err):
			continue
		case err != nil:
			s.Logger.Warn("messaging: group message push failed",
				slog.Int64("group_thread_id", thread.ID),
				slog.String("error", err.Error()),
			)
		}
	}
}

func (s *Service) mailGroupMessage(ctx context.Context, thread *deliveryPorts.ParentGroupThread, messageID int64, accounts []int64) error {
	if s.Outbox == nil || s.GuardianProfiles == nil || s.ParentsURL == "" {
		return nil
	}
	if s.Preferences != nil {
		remaining, err := s.Preferences.FilterNotOptedOut(ctx, deliveryPorts.TypeParentMessage, accounts)
		if err != nil {
			// A consent lookup that did not answer is not a yes.
			s.Logger.Warn("messaging: group message e-mail consent check failed",
				slog.Int64("group_thread_id", thread.ID),
				slog.String("error", err.Error()),
			)
			return nil
		}
		accounts = remaining
	}
	if len(accounts) == 0 {
		return nil
	}
	schoolName, logoURL := s.resolveSchoolBrand(ctx, thread.TenantID)
	link := strings.TrimRight(s.ParentsURL, "/") + "/messages/group/" + strconv.FormatInt(thread.ID, 10)
	for _, accountID := range accounts {
		if err := s.mailGroupRecipient(ctx, thread, messageID, accountID, schoolName, logoURL, link); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) mailGroupRecipient(ctx context.Context, thread *deliveryPorts.ParentGroupThread, messageID, accountID int64, schoolName, logoURL, link string) error {
	profile, err := s.GuardianProfiles.FindMessageGuardianProfile(ctx, accountID)
	if err != nil {
		s.Logger.Warn("messaging: load guardian profile for group e-mail failed",
			slog.Int64("group_thread_id", thread.ID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	if profile == nil || profile.Email == nil || strings.TrimSpace(*profile.Email) == "" {
		return nil
	}
	locale := s.deliveryPolicy.DefaultLocale()
	if profile.PortalLocale != nil {
		locale = s.deliveryPolicy.NormalizeLocale(*profile.PortalLocale)
	}
	request := deliveryPorts.MailEnqueueInput{
		Kind:              deliveryPorts.MailKindParentMessage,
		RelatedEntityType: "parent_group_message",
		RelatedEntityID:   messageID,
		IdempotencyKey:    "parent_group_message:" + strconv.FormatInt(messageID, 10) + ":" + strconv.FormatInt(accountID, 10),
		Payload: map[string]json.RawMessage{
			messagePayloadRecipient:   deliveryPorts.JSONString(strings.TrimSpace(*profile.Email)),
			messagePayloadFirstName:   deliveryPorts.JSONString(profile.FirstName),
			messagePayloadLastName:    deliveryPorts.JSONString(profile.LastName),
			messagePayloadLocale:      deliveryPorts.JSONString(locale),
			messagePayloadSchoolName:  deliveryPorts.JSONString(schoolName),
			messagePayloadMessagesURL: deliveryPorts.JSONString(link),
			messagePayloadLogoURL:     deliveryPorts.JSONString(logoURL),
			messagePayloadMotoLogoURL: deliveryPorts.JSONString(s.deliveryPolicy.MotoLogoURL(s.ParentsURL)),
		},
	}.Public()
	if err := s.Outbox.EnqueueMail(ctx, request); err != nil {
		return fmt.Errorf("messaging: enqueue group message e-mail: %w", err)
	}
	return nil
}

func (s *Service) guardianLocale(ctx context.Context, accountID int64) string {
	if s.GuardianProfiles == nil {
		return ""
	}
	profile, err := s.GuardianProfiles.FindMessageGuardianProfile(ctx, accountID)
	if err != nil || profile == nil || profile.PortalLocale == nil {
		return ""
	}
	return *profile.PortalLocale
}

// broadcastGroupThreadAfterCommit wakes the school's staff tabs and the
// parents' tabs after commit. The event carries no family data; parents
// refetch their own list.
func (s *Service) broadcastGroupThreadAfterCommit(ctx context.Context, thread *deliveryPorts.ParentGroupThread, accountIDs []int64) {
	if thread == nil {
		return
	}
	tenantID := thread.TenantID
	s.runtime.RegisterAfterCommit(ctx, func() {
		s.broadcastValues(tenantID, 0, 0, 0)
		if s.Broadcaster == nil {
			return
		}
		event := realtime.NewParentMessageEvent(0, 0, 0)
		for _, accountID := range accountIDs {
			if err := s.Broadcaster.BroadcastToGuardian(tenantID, accountID, event); err != nil {
				s.Logger.Warn("messaging: failed to broadcast group thread update",
					slog.Int64("group_thread_id", thread.ID),
					slog.Int64("guardian_account_id", accountID),
					slog.String("error", err.Error()),
				)
			}
		}
	})
}

func recipientAccountIDs(recipients []deliveryPorts.ParentGroupRecipient) []int64 {
	accountIDs := make([]int64, 0, len(recipients))
	seen := make(map[int64]struct{}, len(recipients))
	for _, recipient := range recipients {
		if recipient.GuardianAccountID <= 0 {
			continue
		}
		if _, ok := seen[recipient.GuardianAccountID]; ok {
			continue
		}
		seen[recipient.GuardianAccountID] = struct{}{}
		accountIDs = append(accountIDs, recipient.GuardianAccountID)
	}
	return accountIDs
}
