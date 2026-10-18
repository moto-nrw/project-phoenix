//go:build testhooks

package api

import (
	"context"
	"time"

	owner0 "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
)

func ParentStoreChildByID(children []*communication.PortalAnnouncementLetterChildStatus, studentID int64) *communication.PortalAnnouncementLetterChildStatus {
	for _, c := range children {
		if c.StudentID == studentID {
			return c
		}
	}
	return nil
}
func ParentStoreRecipientByProfile(recipients []*communication.PortalAnnouncementDeliveryRecipient, profileID int64) *communication.PortalAnnouncementDeliveryRecipient {
	for _, r := range recipients {
		if r.GuardianProfileID == profileID {
			return r
		}
	}
	return nil
}

// ParentStoreReminderAnnouncement creates an announcement carrying a reminder at
// reminderAt. published decides whether it is live; expiresAt is optional.
func ParentStoreReminderAnnouncement(t ParentStoreFixtureChecks, ctx context.Context, databaseTimestamp func() time.Time, repo parentstore.ParentAnnouncementRepository, createdBy, tenantID int64, title string, reminderAt time.Time, published bool, expiresAt *time.Time) *communication.PortalParentAnnouncement {
	t.Helper()
	text := "Kurz: " + title
	a := &communication.PortalParentAnnouncement{
		Title:        title,
		Body:         "Testtext",
		Priority:     "info",
		Active:       true,
		CreatedBy:    createdBy,
		ExpiresAt:    expiresAt,
		ReminderAt:   &reminderAt,
		ReminderText: &text,
	}
	a.TenantID = tenantID
	t.NoError(repo.Create(ctx, a))
	t.NoError(repo.ReplaceTargets(ctx, tenantID, a.ID, []*communication.PortalParentAnnouncementTarget{{TargetType: "school_all"}}))
	if published {
		publishedAt := databaseTimestamp().Add(-7 * 24 * time.Hour)
		t.NoError(repo.SetPublished(ctx, a.ID, &publishedAt))
	}
	t.Cleanup(func() {
		_ = repo.Delete(ctx, a.ID)
	})
	return a
}
func ParentStoreAnnouncementIDsOf(rows []*communication.PortalParentAnnouncement) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

// ParentStorePollAnnouncement publishes a poll with two options and returns it plus the
// option ids in display order.
func ParentStorePollAnnouncement(t ParentStoreFixtureChecks, ctx context.Context, databaseTimestamp func() time.Time, repo parentstore.ParentAnnouncementRepository, createdBy, tenantID int64, responseType string) (*communication.PortalParentAnnouncement, []int64) {
	t.Helper()
	a := &communication.PortalParentAnnouncement{
		Title:        "Umfrage",
		Body:         "Bitte antworten",
		Priority:     "info",
		Active:       true,
		CreatedBy:    createdBy,
		ResponseType: responseType,
	}
	a.TenantID = tenantID
	t.NoError(repo.Create(ctx, a))
	t.NoError(repo.ReplaceTargets(ctx, tenantID, a.ID, []*communication.PortalParentAnnouncementTarget{{TargetType: "school_all"}}))
	t.NoError(repo.ReplaceOptions(ctx, tenantID, a.ID, []*communication.PortalParentAnnouncementOption{{Label: "Ja"}, {Label: "Nein"}}))
	options, err := repo.ListOptions(ctx, a.ID)
	t.NoError(err)
	t.Len(options, 2)
	now := databaseTimestamp()
	t.NoError(repo.SetPublished(ctx, a.ID, &now))
	a.PublishedAt = &now
	t.Cleanup(func() {
		_ = repo.Delete(ctx, a.ID)
	})
	return a, []int64{options[0].ID, options[1].ID}
}

// ParentStorePublishedAnnouncement creates an announcement with the given targets, publishes
// it, and registers cleanup. created_by is the chain account (a real auth.account).
func ParentStorePublishedAnnouncement(t ParentStoreFixtureChecks, ctx context.Context, databaseTimestamp func() time.Time, repo parentstore.ParentAnnouncementRepository, createdBy, tenantID int64, title string, targets []*communication.PortalParentAnnouncementTarget) *communication.PortalParentAnnouncement {
	t.Helper()
	a := &communication.PortalParentAnnouncement{Title: title, Body: "Testtext", Priority: "info", Active: true, CreatedBy: createdBy}
	a.TenantID = tenantID
	t.NoError(repo.Create(ctx, a))
	t.NoError(repo.ReplaceTargets(ctx, tenantID, a.ID, targets))
	now := databaseTimestamp()
	t.NoError(repo.SetPublished(ctx, a.ID, &now))
	a.PublishedAt = &now
	t.Cleanup(func() {
		_ = repo.Delete(ctx, a.ID)
	})
	return a
}
func ParentStoreAnnouncementIDs(items []*communication.PortalAnnouncementFeedItem) []int64 {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}
func ParentStoreThread(tenantID int64, studentID, guardianAccountID int64) *communication.PortalParentMessageThread {
	t := &communication.PortalParentMessageThread{StudentID: studentID, GuardianAccountID: guardianAccountID}
	t.TenantID = tenantID
	return t
}
func ParentStoreMessage(tenantID int64, threadID, studentID, accountID int64, kind, body string) *communication.PortalParentMessage {
	m := &communication.PortalParentMessage{
		ThreadID:        threadID,
		StudentID:       studentID,
		SenderAccountID: accountID,
		SenderKind:      kind,
		SenderName:      "Test",
		Body:            body,
	}
	m.TenantID = tenantID
	return m
}

// ParentStoreRequestCreatedPill builds a "request created" event pill as the emitter
// stores it: a system event attributed to the GUARDIAN side (event_actor_kind),
// so it counts as unread to a staff reader. actorAccountID is the submitting
// guardian's account (the emitter stamps the actor in sender_account_id).
func ParentStoreRequestCreatedPill(tenantID int64, threadID, studentID, actorAccountID int64, at time.Time) *communication.PortalParentMessage {
	m := &communication.PortalParentMessage{
		ThreadID:        threadID,
		StudentID:       studentID,
		SenderAccountID: actorAccountID,
		SenderKind:      "system",
		SenderName:      "System",
		Body:            "Anfrage gestellt",
		Kind:            "event",
		EventType:       "request_created",
		EventActorKind:  owner0.ParentMessageSenderGuardian,
	}
	m.CreatedAt, m.UpdatedAt = at, at
	m.TenantID = tenantID
	return m
}

type ParentStoreAnnouncementInput struct {
	ID                      int64
	CreatedAt               time.Time
	UpdatedAt               time.Time
	TenantID                int64
	Title                   string
	Body                    string
	Priority                string
	LinkURL                 *string
	RequiresAcknowledgement bool
	SendEmail               bool
	PublishedAt             *time.Time
	ExpiresAt               *time.Time
	Active                  bool
	CreatedBy               int64
	ResponseType            string
	ResponseDeadline        *time.Time
	DeliveryMode            string
	EmailAudience           string
	SystemKind              *string
	ReminderAt              *time.Time
	ReminderText            *string
	ReminderSentAt          *time.Time
	Poll                    bool
	DeclarationMode         bool
}

func ParentStoreAnnouncementValue(v ParentStoreAnnouncementInput) *communication.PortalParentAnnouncement {
	return &communication.PortalParentAnnouncement{
		ID:                      v.ID,
		CreatedAt:               v.CreatedAt,
		UpdatedAt:               v.UpdatedAt,
		TenantID:                v.TenantID,
		Title:                   v.Title,
		Body:                    v.Body,
		Priority:                v.Priority,
		LinkURL:                 v.LinkURL,
		RequiresAcknowledgement: v.RequiresAcknowledgement,
		SendEmail:               v.SendEmail,
		PublishedAt:             v.PublishedAt,
		ExpiresAt:               v.ExpiresAt,
		Active:                  v.Active,
		CreatedBy:               v.CreatedBy,
		ResponseType:            v.ResponseType,
		ResponseDeadline:        v.ResponseDeadline,
		DeliveryMode:            v.DeliveryMode,
		EmailAudience:           v.EmailAudience,
		SystemKind:              v.SystemKind,
		ReminderAt:              v.ReminderAt,
		ReminderText:            v.ReminderText,
		ReminderSentAt:          v.ReminderSentAt,
		Poll:                    v.Poll,
		DeclarationMode:         v.DeclarationMode,
	}
}

type ParentStoreTargetInput struct {
	TenantID       int64
	ID             int64
	AnnouncementID int64
	TargetType     string
	TargetRefID    *int64
	TargetRefText  *string
	CreatedAt      time.Time
}

func ParentStoreTargetValue(v ParentStoreTargetInput) *communication.PortalParentAnnouncementTarget {
	return &communication.PortalParentAnnouncementTarget{
		TenantID:       v.TenantID,
		ID:             v.ID,
		AnnouncementID: v.AnnouncementID,
		TargetType:     v.TargetType,
		TargetRefID:    v.TargetRefID,
		TargetRefText:  v.TargetRefText,
		CreatedAt:      v.CreatedAt,
	}
}
func ParentStoreTargetValues(values []ParentStoreTargetInput) []*communication.PortalParentAnnouncementTarget {
	if values == nil {
		return nil
	}
	out := make([]*communication.PortalParentAnnouncementTarget, len(values))
	for i, v := range values {
		out[i] = ParentStoreTargetValue(v)
	}
	return out
}

type ParentStoreFeedScopeInput struct {
	TenantIDs           []int64
	SystemOnlyTenantIDs []int64
}

func ParentStoreFeedScopeValue(v ParentStoreFeedScopeInput) communication.PortalAnnouncementFeedScope {
	return communication.PortalAnnouncementFeedScope{TenantIDs: v.TenantIDs, SystemOnlyTenantIDs: v.SystemOnlyTenantIDs}
}

func ParentStoreRecipientByAccount(list []*communication.PortalAnnouncementRecipientStatus, accountID int64) *communication.PortalAnnouncementRecipientStatus {
	for _, r := range list {
		if r.AccountID == accountID {
			return r
		}
	}
	return nil
}
func ParentStorePublishedConflict() error { return parentstore.ParentAnnouncementPublishedError() }

func ParentStoreEmptyStats() *communication.PortalAnnouncementStats {
	return &communication.PortalAnnouncementStats{}
}

// ParentStoreFixtureChecks keeps assertions and lifecycle in the calling test.
type ParentStoreFixtureChecks struct {
	Helper  func()
	Cleanup func(func())
	NoError func(error)
	Len     func(any, int)
}
