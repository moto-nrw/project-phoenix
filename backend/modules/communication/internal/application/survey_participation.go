package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// staffSurveyDeepLink is the staff portal page that lists the open surveys.
const staffSurveyDeepLink = "/umfragen"

// StaffSurveysToAnswer lists the open staff surveys of the school for a staff
// member. Someone outside the staff audience gets none.
func (s *surveyService) StaffSurveysToAnswer(ctx context.Context, accountID int64) ([]communication.SurveyToAnswer, error) {
	if s.staff == nil {
		return nil, errors.New("survey: staff audience is not wired")
	}
	staff, err := s.staff.ListAddressableStaff(ctx, 0)
	if err != nil {
		return nil, fmt.Errorf("survey: list staff audience: %w", err)
	}
	if !inStaffAudience(staff, accountID) {
		return []communication.SurveyToAnswer{}, nil
	}
	return s.store.OpenStaffSurveys(ctx, accountID, s.now())
}

// StaffSurveyForm returns a published staff survey for answering.
func (s *surveyService) StaffSurveyForm(ctx context.Context, accountID, surveyID int64) (*communication.SurveyForm, error) {
	survey, err := s.staffSurveyFor(ctx, accountID, surveyID)
	if err != nil {
		return nil, err
	}
	return s.form(ctx, survey, accountID)
}

// SubmitStaffSurvey stores a staff member's answers to an open staff survey.
func (s *surveyService) SubmitStaffSurvey(ctx context.Context, accountID, surveyID int64, answers []communication.SurveyAnswerInput) error {
	survey, err := s.staffSurveyFor(ctx, accountID, surveyID)
	if err != nil {
		return err
	}
	return s.submit(ctx, survey, accountID, answers)
}

func (s *surveyService) staffSurveyFor(ctx context.Context, accountID, surveyID int64) (*communication.Survey, error) {
	survey, err := s.store.FindSurvey(ctx, surveyID, false)
	if err != nil {
		return nil, err
	}
	if survey == nil || survey.Audience != communication.SurveyAudienceStaff || survey.PublishedAt == nil {
		return nil, communication.ErrSurveyNotFound
	}
	member, err := s.isStaffMember(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, communication.ErrSurveyNotFound
	}
	return survey, nil
}

func (s *surveyService) isStaffMember(ctx context.Context, accountID int64) (bool, error) {
	staff, err := s.staffAudience(ctx)
	if err != nil {
		return false, err
	}
	return inStaffAudience(staff, accountID), nil
}

// inStaffAudience reports whether the account is one of the staff a staff
// survey asks; accounts of the school portal only are not.
func inStaffAudience(staff []*communication.AddressableStaffRecord, accountID int64) bool {
	if accountID <= 0 {
		return false
	}
	for _, member := range staff {
		if member != nil && member.AccountID == accountID && member.RoleKind != surveyLehrkraftRoleKind {
			return true
		}
	}
	return false
}

// PortalSurveyForm returns the parent survey an announcement carries.
func (s *surveyService) PortalSurveyForm(ctx context.Context, tenantID, announcementID, accountID int64) (*communication.SurveyForm, error) {
	survey, err := s.portalSurvey(ctx, tenantID, announcementID)
	if err != nil {
		return nil, err
	}
	return s.form(ctx, survey, accountID)
}

// SubmitPortalSurvey stores a guardian's answers: one submission per
// guardian account, whatever number of children the survey reaches.
func (s *surveyService) SubmitPortalSurvey(ctx context.Context, tenantID, announcementID, accountID int64, answers []communication.SurveyAnswerInput) error {
	survey, err := s.portalSurvey(ctx, tenantID, announcementID)
	if err != nil {
		return err
	}
	return s.submit(ctx, survey, accountID, answers)
}

func (s *surveyService) portalSurvey(ctx context.Context, tenantID, announcementID int64) (*communication.Survey, error) {
	survey, err := s.store.FindPortalSurvey(ctx, tenantID, announcementID)
	if err != nil {
		return nil, err
	}
	if survey == nil {
		return nil, communication.ErrSurveyNotFound
	}
	return survey, nil
}

func (s *surveyService) form(ctx context.Context, survey *communication.Survey, accountID int64) (*communication.SurveyForm, error) {
	participated, err := s.store.HasParticipated(ctx, survey.TenantID, survey.ID, accountID)
	if err != nil {
		return nil, err
	}
	school, err := s.announcements.repo.SchoolName(ctx, survey.TenantID)
	if err != nil {
		return nil, fmt.Errorf("survey: resolve school name: %w", err)
	}
	return &communication.SurveyForm{Survey: *survey, SchoolName: school, Participated: participated}, nil
}

// submit stores one submission. The log line names the survey only: neither
// the account nor an answer appears in any log.
func (s *surveyService) submit(ctx context.Context, survey *communication.Survey, accountID int64, answers []communication.SurveyAnswerInput) error {
	if survey.Status(s.now()) != communication.SurveyStatusOpen {
		return communication.ErrSurveyNotOpen
	}
	clean, err := normalizeSurveyAnswers(survey, answers)
	if err != nil {
		return err
	}
	recorded, err := s.store.RecordSubmission(ctx, survey.TenantID, survey.ID, accountID, !survey.Anonymous, clean)
	if err != nil {
		return err
	}
	if !recorded {
		return communication.ErrSurveyAlreadyAnswered
	}
	s.logger.Info("survey answer recorded", slog.Int64("survey_id", survey.ID))
	return nil
}

// notifyStaff pushes a staff survey to the staff audience that opted into
// survey notifications. narrow, if set, picks who of the audience is asked.
func (s *surveyService) notifyStaff(ctx context.Context, survey *communication.Survey, keyFormat string, reminder bool, narrow func([]int64) []int64) (int, error) {
	n := s.announcements.notifier
	if n == nil {
		return 0, nil
	}
	staff, err := s.staffAudience(ctx)
	if err != nil {
		return 0, err
	}
	accountIDs := make([]int64, 0, len(staff))
	for _, member := range staff {
		accountIDs = append(accountIDs, member.AccountID)
	}
	if narrow != nil {
		accountIDs = narrow(accountIDs)
	}
	if len(accountIDs) == 0 || s.announcements.preferences == nil {
		return 0, nil
	}
	accountIDs, err = s.announcements.preferences.FilterOptedIn(ctx, TypeStaffSurvey, accountIDs)
	if err != nil {
		return 0, fmt.Errorf("survey: filter opted-in staff: %w", err)
	}
	if len(accountIDs) == 0 {
		return 0, nil
	}
	title, body := "Neue Umfrage", fmt.Sprintf("Bitte nehmen Sie an der Umfrage „%s“ teil.", survey.Title)
	if reminder {
		title = "Erinnerung an eine Umfrage"
		body = fmt.Sprintf("Die Umfrage „%s“ läuft noch bis zum %s.", survey.Title, survey.ClosesAt.In(calendar.Berlin).Format("02.01.2006"))
	}
	err = n.Notify(ctx, NotificationEvent{
		Type: TypeStaffSurvey, IdempotencyKey: fmt.Sprintf(keyFormat, survey.ID),
		RelatedType: "staff_survey", RelatedID: survey.ID,
		Title: title, Body: body, DeepLink: staffSurveyDeepLink, Priority: PriorityNormal,
		Audience: NotificationAudience{TenantID: survey.TenantID, Scope: ScopeStaff, StaffAccountIDs: accountIDs},
	}.Public())
	if s.announcements.deliveryPolicy.IsNotificationsDisabled(err) || s.announcements.deliveryPolicy.IsOutsideActiveWindow(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("survey: notify staff: %w", err)
	}
	return len(accountIDs), nil
}

// DeleteExpiredSurveys deletes the surveys of one audience past the window,
// each with its carrier announcement.
func (s *surveyService) DeleteExpiredSurveys(ctx context.Context, audience string, retentionDays int) (int, error) {
	if audience != communication.SurveyAudienceParents && audience != communication.SurveyAudienceStaff {
		return 0, fmt.Errorf("survey: unknown audience %q", audience)
	}
	if retentionDays <= 0 {
		return 0, fmt.Errorf("survey: retention window must be positive, got %d", retentionDays)
	}
	expired, err := s.store.ExpiredSurveys(ctx, audience, surveyRetentionCutoff(s.now(), retentionDays))
	if err != nil {
		return 0, err
	}
	for _, survey := range expired {
		if survey.AnnouncementID != nil {
			if err := s.announcements.deleteSurveyCarrier(ctx, *survey.AnnouncementID); err != nil {
				return 0, err
			}
		}
		if err := s.store.DeleteSurvey(ctx, survey.ID); err != nil {
			return 0, err
		}
	}
	return len(expired), nil
}

// surveyRetentionCutoff is the start of the Berlin calendar day retentionDays
// before now, like the window of the parent announcements: a survey closed
// before it is due. The day count does not shift by an hour across a change
// of daylight saving time, whatever the zone of the process.
func surveyRetentionCutoff(now time.Time, retentionDays int) time.Time {
	return calendar.DateFromTime(now).AddDays(-retentionDays).BerlinMidnight()
}

// namedRespondents labels the stored answers of a named survey for the
// school: the person's name and the option texts.
func namedRespondents(survey *communication.Survey, responses []communication.SurveyNamedResponse, names map[int64]string) []communication.SurveyRespondent {
	labels := map[int64]string{}
	for _, q := range survey.Questions {
		for _, o := range q.Options {
			labels[o.ID] = o.Label
		}
	}
	out := make([]communication.SurveyRespondent, 0, len(responses))
	for _, response := range responses {
		name := names[response.AccountID]
		if name == "" {
			name = "Unbekannt"
		}
		respondent := communication.SurveyRespondent{Name: name, Answers: make([]communication.SurveyRespondentAnswer, 0, len(response.Answers))}
		for _, a := range response.Answers {
			answer := communication.SurveyRespondentAnswer{QuestionID: a.QuestionID, Options: []string{}, ScaleValue: a.ScaleValue, Text: a.Text}
			for _, id := range a.OptionIDs {
				answer.Options = append(answer.Options, labels[id])
			}
			respondent.Answers = append(respondent.Answers, answer)
		}
		out = append(out, respondent)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
