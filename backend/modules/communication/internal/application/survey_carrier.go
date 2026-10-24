package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// A parent survey reaches the families as a system-authored parent
// announcement (#4781), like the cancellation notice of #2601: the
// announcement carries the targets, the feed entry, the push and the
// e-mail. The survey keeps the questions and the answers. Being a system row,
// the announcement cannot be edited, unpublished or deleted on its own in the
// announcement routes; the survey drives it.

// StaffParentAnnouncementSystemKindSurvey marks the carrier announcement.
const StaffParentAnnouncementSystemKindSurvey = "survey"

const (
	relatedEntityTypeSurveyReminder = "parent_survey_reminder"
	surveyEmailKicker               = "Umfrage"
)

// publishSurveyCarrier creates and publishes the carrier of a parent survey
// in the caller's school transaction and sends its push and, if chosen, its
// e-mail at the survey's own publication time. It returns the announcement id.
func (s *service) publishSurveyCarrier(ctx context.Context, survey *communication.Survey, createdBy int64, now time.Time) (int64, error) {
	kind := StaffParentAnnouncementSystemKindSurvey
	body := survey.Intro
	if body == "" {
		body = surveyDefaultParentBody
	}
	expires := survey.ClosesAt
	a := &StaffParentAnnouncement{
		Title:      survey.Title,
		Body:       body,
		Priority:   StaffParentAnnouncementPriorityInfo,
		SendEmail:  survey.SendEmail,
		ExpiresAt:  &expires,
		Active:     true,
		CreatedBy:  createdBy,
		SystemKind: &kind,
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return 0, fmt.Errorf("survey: create carrier announcement: %w", err)
	}
	targets := make([]*StaffParentAnnouncementTarget, 0, len(survey.Targets))
	for _, t := range survey.Targets {
		targets = append(targets, &StaffParentAnnouncementTarget{TargetType: t.TargetType, TargetRefID: t.RefID, TargetRefText: t.RefText})
	}
	if err := s.repo.ReplaceTargets(ctx, a.GetTenantID(), a.ID, targets); err != nil {
		return 0, fmt.Errorf("survey: set carrier targets: %w", err)
	}
	published, err := s.repo.PublishIfDraft(ctx, a.ID, now)
	if err != nil {
		return 0, fmt.Errorf("survey: publish carrier announcement: %w", err)
	}
	if !published {
		return 0, fmt.Errorf("survey: publish carrier announcement: row %d was not a draft", a.ID)
	}
	a.PublishedAt = &now
	a.Targets = targets
	if err := s.notifyAnnouncementGuardians(ctx, a); err != nil {
		return 0, fmt.Errorf("survey: carrier push: %w", err)
	}
	if err := s.enqueueAnnouncementEmails(ctx, a); err != nil {
		return 0, fmt.Errorf("survey: carrier e-mails: %w", err)
	}
	return a.ID, nil
}

// deleteSurveyCarrier removes the carrier with its pending mail and pushes.
// A carrier the announcement retention already removed is no error.
func (s *service) deleteSurveyCarrier(ctx context.Context, announcementID int64) error {
	a, err := s.repo.FindByIDForUpdate(ctx, announcementID)
	if err != nil {
		return fmt.Errorf("survey: load carrier announcement: %w", err)
	}
	if a == nil {
		return nil
	}
	return s.deleteLocked(ctx, a)
}

// remindSurveyGuardians nudges every guardian of the carrier's audience who
// has not taken part: one push and one e-mail per person. who took part comes
// from the survey; the reminder never tells the school who that is.
func (s *service) remindSurveyGuardians(ctx context.Context, survey *communication.Survey, participated map[int64]bool) (int, error) {
	if survey.AnnouncementID == nil {
		return 0, nil
	}
	a, err := s.repo.FindByID(ctx, *survey.AnnouncementID)
	if err != nil {
		return 0, fmt.Errorf("survey: load carrier announcement: %w", err)
	}
	if a == nil {
		return 0, nil
	}
	audience, err := s.repo.ResolveDeliveryRecipients(ctx, a.GetTenantID(), a.ID)
	if err != nil {
		return 0, fmt.Errorf("survey: resolve reminder audience: %w", err)
	}
	recipients := make([]*StaffAnnouncementPollReminderRecipient, 0, len(audience))
	for _, r := range audience {
		if r.AccountID == nil || !r.HasPortalAccess || participated[*r.AccountID] {
			continue
		}
		recipients = append(recipients, &StaffAnnouncementPollReminderRecipient{
			AccountID: *r.AccountID, Email: r.Email, FirstName: r.FirstName, LastName: r.LastName, PortalLocale: r.PortalLocale,
		})
	}
	if len(recipients) == 0 {
		return 0, nil
	}
	intro := fmt.Sprintf("die Umfrage der Schule läuft noch bis zum %s. Sie finden sie im Eltern-Portal.",
		survey.ClosesAt.In(calendar.Berlin).Format("02.01.2006"))
	emailed, err := s.enqueueReminderEmails(ctx, a, recipients, pollReminderEmailKicker, intro, s.parentsURL)
	if err != nil {
		return 0, err
	}
	pushed, err := s.pushPollReminder(ctx, a, recipients)
	if err != nil {
		return 0, err
	}
	delivered := make(map[int64]struct{}, len(recipients))
	for _, id := range append(pushed, emailed...) {
		delivered[id] = struct{}{}
	}
	s.logger.Info("parent survey reminder sent",
		slog.Int64("survey_id", survey.ID),
		slog.Int("recipient_count", len(delivered)),
	)
	return len(delivered), nil
}

// surveyMailSpec words the publish mail of a survey carrier; any other
// announcement keeps its spec.
func surveyMailSpec(a *StaffParentAnnouncement, spec mailSpec) mailSpec {
	if !isSurveyCarrier(a) {
		return spec
	}
	spec.kicker = surveyEmailKicker
	intro := "die Schule bittet Sie um Ihre Meinung. Die Umfrage finden Sie im Eltern-Portal."
	if a.ExpiresAt != nil {
		intro = fmt.Sprintf("die Schule bittet Sie bis zum %s um Ihre Meinung. Die Umfrage finden Sie im Eltern-Portal.",
			a.ExpiresAt.In(calendar.Berlin).Format("02.01.2006"))
	}
	spec.intro = func(string) string { return intro }
	return spec
}

// withoutSurveyCarriers drops survey carriers from the announcement list:
// they belong to the survey list, where they can be managed.
func withoutSurveyCarriers(rows []*StaffParentAnnouncement) []*StaffParentAnnouncement {
	out := rows[:0:0]
	for _, a := range rows {
		if isSurveyCarrier(a) {
			continue
		}
		out = append(out, a)
	}
	return out
}

// isSurveyCarrier tells the system announcement of a parent survey. It is
// none of the staff's announcement routes: its read times per guardian would
// line up with the answers of an anonymous survey, so those routes do not
// find it.
func isSurveyCarrier(a *StaffParentAnnouncement) bool {
	return a.IsSystem() && *a.SystemKind == StaffParentAnnouncementSystemKindSurvey
}
