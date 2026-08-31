package api

import (
	"context"
	"fmt"
	"strconv"
)

type seedParentEngagementStep struct{}

func (seedParentEngagementStep) Name() string { return "Seeding parent engagement" }

func (seedParentEngagementStep) Run(ctx context.Context, rt *Runtime) error {
	if rt == nil || rt.Adapter == nil || rt.Client == nil || len(rt.Parents) == 0 || len(rt.Parents[0].StudentIDs) == 0 {
		return fmt.Errorf("parent engagement prerequisites not available")
	}
	parent := rt.Parents[0]
	auth, err := rt.Adapter.LoginParent(ctx, parent.Email, parent.Password)
	if err != nil {
		return fmt.Errorf("login parent for engagement demo: %w", err)
	}
	studentID := parent.StudentIDs[0]
	if err := seedParentNotificationPreference(rt, auth); err != nil {
		return err
	}
	if err := seedParentConversation(rt, auth, studentID); err != nil {
		return err
	}
	if err := seedParentGuardianChanges(rt, auth, studentID); err != nil {
		return err
	}
	if err := seedParentPhotoConsentHistory(rt, auth, studentID); err != nil {
		return err
	}
	if err := seedParentMasterData(rt, auth, studentID); err != nil {
		return err
	}
	rt.Client.BindAuth(rt.TenantAuth)
	fmt.Println("  1 parent preference, 1 conversation, audited contact/consent changes and 1 master-data request created")
	return nil
}

func seedParentPhotoConsentHistory(rt *Runtime, auth AuthRef, studentID int64) error {
	if _, err := rt.Client.PutWithAuth(rt.TenantAuth, fmt.Sprintf("/api/students/%d", studentID), map[string]any{
		"photo_consent_given": true,
	}); err != nil {
		return fmt.Errorf("seed granted photo consent: %w", err)
	}
	if _, err := rt.Client.DeleteWithAuth(auth, fmt.Sprintf("/parent/me/children/%d/consents/photo", studentID)); err != nil {
		return fmt.Errorf("seed withdrawn parent photo consent: %w", err)
	}
	return nil
}

func seedParentNotificationPreference(rt *Runtime, auth AuthRef) error {
	if _, err := rt.Client.PutWithAuth(auth, "/parent/me/notification-preferences/parent_message", map[string]any{
		"enabled": true,
	}); err != nil {
		return fmt.Errorf("seed parent notification preference: %w", err)
	}
	return nil
}

func seedParentConversation(rt *Runtime, auth AuthRef, studentID int64) error {
	path := fmt.Sprintf("/parent/me/messages/children/%d", studentID)
	raw, err := rt.Client.PostWithAuth(auth, path, map[string]any{
		"body": "Können Sie bitte prüfen, ob die neue Abholzeit eingetragen ist?",
	})
	if err != nil {
		return fmt.Errorf("seed parent message: %w", err)
	}
	threadID, messageID, err := parseThreadMessageIDs(raw)
	if err != nil {
		return fmt.Errorf("parse parent conversation: %w", err)
	}
	if _, err := rt.Client.PostWithAuth(rt.TenantAuth, fmt.Sprintf("/api/messages/threads/%d", threadID), map[string]any{
		"body": "Die neue Abholzeit ist eingetragen.", "handled_up_to_message_id": strconv.FormatInt(messageID, 10),
	}); err != nil {
		return fmt.Errorf("seed staff reply: %w", err)
	}
	if _, err := rt.Client.GetWithAuth(auth, path); err != nil {
		return fmt.Errorf("read seeded parent conversation: %w", err)
	}
	return nil
}

func seedParentGuardianChanges(rt *Runtime, auth AuthRef, studentID int64) error {
	contactRaw, err := rt.Client.PostWithAuth(auth, fmt.Sprintf("/parent/me/children/%d/guardians", studentID), map[string]any{
		"first_name": "Rita", "last_name": "Abholkontakt", "email": "rita.abholkontakt@example.test",
		"address_street": "Nebenweg 3", "address_postal_code": "50667", "address_city": "Köln",
		"phones":            []map[string]any{{"phone_number": "+49 221 555 778", "phone_type": "mobile", "label": "Privat", "is_primary": true}},
		"relationship_type": "relative", "can_pickup": true, "is_emergency_contact": false,
	})
	if err != nil {
		return fmt.Errorf("seed parent-managed pickup contact: %w", err)
	}
	contactID, err := parseGuardianProfileID(contactRaw)
	if err != nil {
		return fmt.Errorf("parse parent-managed pickup contact: %w", err)
	}
	if _, err := rt.Client.PutWithAuth(auth,
		fmt.Sprintf("/parent/me/children/%d/guardians/%d/pickup", studentID, contactID),
		map[string]any{"can_pickup": false, "is_emergency_contact": true, "pickup_notes": "Nur nach telefonischer Rücksprache"},
	); err != nil {
		return fmt.Errorf("update parent-managed pickup contact: %w", err)
	}
	return withTemporarySeedSetting(rt, rt.TenantAuth, "guardians.parent_invite_mode", "staff_approval", "direct", func() error {
		if _, err := rt.Client.PostWithAuth(auth, fmt.Sprintf("/parent/me/children/%d/related-accounts", studentID), map[string]any{
			"email": "rita.abholkontakt@example.test", "confirm_role_upgrade": true,
		}); err != nil {
			return fmt.Errorf("seed pending guardian role upgrade: %w", err)
		}
		return nil
	})
}

func seedParentMasterData(rt *Runtime, auth AuthRef, studentID int64) error {
	if _, err := rt.Client.PatchWithAuth(auth,
		fmt.Sprintf("/parent/me/children/%d/master-data/guardian_profile/preferred_contact_method", studentID),
		map[string]any{"value": "phone"},
	); err != nil {
		return fmt.Errorf("seed parent contact preference: %w", err)
	}
	if _, err := rt.Client.PostWithAuth(auth,
		fmt.Sprintf("/parent/me/children/%d/master-data/requests", studentID),
		map[string]any{
			"changes": []map[string]any{
				{"target": "person", "field_key": "first_name", "value": "Felix-Max"},
			},
			"recipient_guardian_profile_ids": []string{},
		},
	); err != nil {
		return fmt.Errorf("seed parent master-data request: %w", err)
	}
	return nil
}

func parseGuardianProfileID(raw []byte) (int64, error) {
	var envelope struct {
		Data struct {
			GuardianProfileID string `json:"guardian_profile_id"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &envelope); err != nil {
		return 0, err
	}
	id, err := strconv.ParseInt(envelope.Data.GuardianProfileID, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid guardian_profile_id %q", envelope.Data.GuardianProfileID)
	}
	return id, nil
}

func parseThreadMessageIDs(raw []byte) (int64, int64, error) {
	var envelope struct {
		Data struct {
			ThreadID string `json:"thread_id"`
			Messages []struct {
				ID string `json:"id"`
			} `json:"messages"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &envelope); err != nil {
		return 0, 0, err
	}
	id, err := strconv.ParseInt(envelope.Data.ThreadID, 10, 64)
	if err != nil || id <= 0 {
		return 0, 0, fmt.Errorf("invalid thread_id %q", envelope.Data.ThreadID)
	}
	if len(envelope.Data.Messages) == 0 {
		return 0, 0, fmt.Errorf("parent conversation has no messages")
	}
	messageID, err := strconv.ParseInt(envelope.Data.Messages[len(envelope.Data.Messages)-1].ID, 10, 64)
	if err != nil || messageID <= 0 {
		return 0, 0, fmt.Errorf("invalid message id")
	}
	return id, messageID, nil
}
