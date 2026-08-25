package announcement

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Cancellation notice (#2601).
//
// When a care block is cancelled the families of the booked children get a
// regular parent announcement, authored by the system on behalf of the person
// cancelling. Reusing the announcement gives the notice everything a
// Leistungskatalog asks for without a second delivery system: a feed entry
// that stays, a read state per family, an optional e-mail through the shared
// outbox, and a push. The row is marked with SystemKind so the team list can
// badge it and the parent feed can show it even where the optional news
// feature is off.

// ErrCareCancellationDisabled is returned when the school switched the
// cancellation notice off. Handler maps it to 409.
var ErrCareCancellationDisabled = errors.New("announcement: cancellation notice is disabled for this school")

// The parents app serves the feed at /news; the portal adds its own prefix,
// so the deep link is the browser path. Lock-screen copy lives with the other
// parent copy in notifications.ParentAnnouncementCopy (no child name, no
// block title; the feed entry carries the details).
const careCancelledDeepLink = "/news"

// CareCancellationInput describes one cancelled block's notice.
type CareCancellationInput struct {
	// StudentIDs are the children booked into the cancelled block. Their
	// guardians with portal access form the audience.
	StudentIDs []int64
	// Title and Body are what the person cancelling wrote (or accepted from the
	// prefilled suggestion). The internal cancel reason never reaches here:
	// only text written for the families is sent to the families.
	Title string
	Body  string
	// CreatedBy is the account that cancelled the block; the announcement is
	// attributed to it like a hand-written one.
	CreatedBy int64
}

// CareCancellationResult reports what was sent.
type CareCancellationResult struct {
	Announcement *usersModels.ParentAnnouncement
	// RecipientCount is the number of guardian accounts the notice reaches in
	// the feed, before push consent and e-mail opt-out narrow the channels.
	RecipientCount int
}

// CareCancellationReach is what the cancel dialog shows before anything is
// sent: whether the school allows the notice at all, whether the checkbox
// starts ticked, and how many families a notice for these children reaches.
type CareCancellationReach struct {
	Enabled     bool
	DefaultOn   bool
	FamilyCount int
}

// CareCancellationPublisher is the slice of the announcement service the
// schedule cancel path depends on.
type CareCancellationPublisher interface {
	// PublishCareCancellation creates and publishes the notice inside the
	// caller's tenant transaction. It fails with ErrCareCancellationDisabled
	// when the school gate is off and with ErrValidation on empty text, so the
	// caller can refuse before the cancellation itself is written.
	PublishCareCancellation(ctx context.Context, in CareCancellationInput) (*CareCancellationResult, error)
	// CareCancellationReachFor resolves the dialog preview for the given
	// children. Runs inside the caller's tenant transaction.
	CareCancellationReachFor(ctx context.Context, studentIDs []int64) (*CareCancellationReach, error)
}

func (s *service) careCancellationEnabled(ctx context.Context) (bool, error) {
	if s.settings == nil {
		return false, nil
	}
	return s.settings.ResolveBool(ctx, configModel.KeyNotificationsCareCancelledEnabled)
}

func (s *service) CareCancellationReachFor(ctx context.Context, studentIDs []int64) (*CareCancellationReach, error) {
	enabled, err := s.careCancellationEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("announcement: resolve cancellation notice flag: %w", err)
	}
	reach := &CareCancellationReach{Enabled: enabled}
	if !enabled {
		return reach, nil
	}
	defaultOn, err := s.settings.ResolveBool(ctx, configModel.KeyNotificationsCareCancelledDefaultOn)
	if err != nil {
		return nil, fmt.Errorf("announcement: resolve cancellation notice default: %w", err)
	}
	reach.DefaultOn = defaultOn
	ids := uniquePositive(studentIDs)
	if len(ids) == 0 {
		return reach, nil
	}
	count, err := s.repo.CountReachableGuardiansForStudents(ctx, tenant.FromContext(ctx), ids)
	if err != nil {
		return nil, fmt.Errorf("announcement: count reachable guardians: %w", err)
	}
	reach.FamilyCount = count
	return reach, nil
}

func (s *service) PublishCareCancellation(ctx context.Context, in CareCancellationInput) (*CareCancellationResult, error) {
	enabled, err := s.careCancellationEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("announcement: resolve cancellation notice flag: %w", err)
	}
	if !enabled {
		return nil, ErrCareCancellationDisabled
	}
	title := strings.TrimSpace(in.Title)
	body := strings.TrimSpace(in.Body)
	if title == "" || len([]rune(title)) > maxTitleLen {
		return nil, fmt.Errorf("%w: title is required and at most %d characters", ErrValidation, maxTitleLen)
	}
	if body == "" || len([]rune(body)) > maxBodyLen {
		return nil, fmt.Errorf("%w: body is required and at most %d characters", ErrValidation, maxBodyLen)
	}
	if in.CreatedBy <= 0 {
		return nil, fmt.Errorf("%w: created_by is required", ErrValidation)
	}
	studentIDs := uniquePositive(in.StudentIDs)
	if len(studentIDs) == 0 {
		return nil, fmt.Errorf("%w: at least one student is required", ErrValidation)
	}
	sendEmail, err := s.settings.ResolveBool(ctx, configModel.KeyNotificationsCareCancelledEmail)
	if err != nil {
		return nil, fmt.Errorf("announcement: resolve cancellation notice e-mail flag: %w", err)
	}

	kind := usersModels.ParentAnnouncementSystemKindCareCancellation
	a := &usersModels.ParentAnnouncement{
		Title:      title,
		Body:       body,
		Priority:   usersModels.ParentAnnouncementPriorityImportant,
		SendEmail:  sendEmail,
		Active:     true,
		CreatedBy:  in.CreatedBy,
		SystemKind: &kind,
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return nil, fmt.Errorf("announcement: create cancellation notice: %w", err)
	}
	targets := make([]*usersModels.ParentAnnouncementTarget, 0, len(studentIDs))
	for _, studentID := range studentIDs {
		id := studentID
		targets = append(targets, &usersModels.ParentAnnouncementTarget{
			TargetType:  usersModels.AnnouncementTargetStudent,
			TargetRefID: &id,
		})
	}
	// Targets can only be set on a draft, so the row is created unpublished
	// and flipped right after. PublishIfDraft is the same atomic flip the
	// hand-written path uses.
	if err := s.repo.ReplaceTargets(ctx, a.GetTenantID(), a.ID, targets); err != nil {
		return nil, fmt.Errorf("announcement: set cancellation notice targets: %w", err)
	}
	now := time.Now()
	published, err := s.repo.PublishIfDraft(ctx, a.ID, now)
	if err != nil {
		return nil, fmt.Errorf("announcement: publish cancellation notice: %w", err)
	}
	if !published {
		return nil, fmt.Errorf("announcement: publish cancellation notice: row %d was not a draft", a.ID)
	}
	a.PublishedAt = &now
	a.Targets = targets

	recipients, err := s.repo.AudienceRecipients(ctx, a.GetTenantID(), a.ID)
	if err != nil {
		return nil, fmt.Errorf("announcement: resolve cancellation notice audience: %w", err)
	}
	s.logger.Info("care cancellation notice published",
		slog.Int64("announcement_id", a.ID),
		slog.Int64("created_by", in.CreatedBy),
		slog.Int("student_count", len(studentIDs)),
		slog.Int("recipient_count", len(recipients)),
		slog.Bool("send_email", sendEmail),
	)
	if err := s.notifyAnnouncementGuardians(ctx, a); err != nil {
		return nil, fmt.Errorf("announcement: cancellation notice push: %w", err)
	}
	if sendEmail {
		if err := s.enqueueAnnouncementEmails(ctx, a); err != nil {
			return nil, fmt.Errorf("announcement: cancellation notice e-mails: %w", err)
		}
	}
	return &CareCancellationResult{Announcement: a, RecipientCount: len(recipients)}, nil
}

// pushShapeFor picks the notification type, the locale-copy kind and the deep
// link for an announcement. Hand-written rows and polls keep their generic
// wording under the announcement consent; a system row rides its own type so a
// family can opt into cancellations separately from news.
func pushShapeFor(a *usersModels.ParentAnnouncement) (notificationType, copyKind, deepLink string) {
	if a.IsSystem() && *a.SystemKind == usersModels.ParentAnnouncementSystemKindCareCancellation {
		return notifications.TypeParentCareCancelled, notifications.ParentCareCancelled, careCancelledDeepLink
	}
	if a.IsPoll() {
		return parentAnnouncementNotificationType, notifications.ParentPollPublished, "/"
	}
	return parentAnnouncementNotificationType, notifications.ParentAnnouncementPublished, "/"
}

func uniquePositive(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
