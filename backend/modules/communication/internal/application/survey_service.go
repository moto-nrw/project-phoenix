package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// SurveyStore is the persistence of the surveys (#4781).
type SurveyStore interface{ communication.SurveyRecords }

// surveyStoreSource is an announcement source that also keeps the surveys:
// the users.* persistence of the same owner over the same database.
type surveyStoreSource interface {
	SurveyRecords() communication.SurveyRecords
}

// SurveyStaff is the school's staff a staff survey asks: the colleagues of
// the team chat (#2598), without accounts that only use the school portal.
type SurveyStaff interface {
	ListAddressableStaff(ctx context.Context, viewerAccountID int64) ([]*communication.AddressableStaffRecord, error)
}

// surveyLehrkraftRoleKind is the role kind of an account that only uses the
// school portal (#2208); a staff survey is answered in the OGS portal.
const surveyLehrkraftRoleKind = "lehrkraft"

// surveyReads are the store reads the capability serves as they are.
type surveyReads interface {
	ListSurveys(ctx context.Context) ([]communication.Survey, error)
	PortalSurveySummaries(ctx context.Context, accountID int64, announcementIDs []int64) (map[int64]communication.PortalSurveySummary, error)
}

type surveyService struct {
	surveyReads
	announcements *service
	store         SurveyStore
	staff         SurveyStaff
	logger        *slog.Logger
	now           func() time.Time
	shuffle       func([]string)
}

// announcementsWithSurveys is the parent announcement capability with the
// surveys of the same owner. Its retention stays reachable the way the
// Worker asks for it.
type announcementsWithSurveys struct {
	communication.ParentAnnouncementCapability
	communication.ParentAnnouncementRetention
	communication.SurveyCapability
}

// WithSurveys adds the surveys (#4781) to the parent announcements. A parent
// survey is carried by a system announcement of the same service; the
// surveys' store comes with the announcement source. Without a staff
// relation the announcements stay without surveys.
func WithSurveys(announcements communication.ParentAnnouncementCapability, source StaffAnnouncementSource, staff SurveyStaff) (communication.ParentAnnouncementCapability, error) {
	if staff == nil {
		return announcements, nil
	}
	facade, ok := announcements.(*parentAnnouncements)
	if !ok {
		return nil, fmt.Errorf("surveys: parent announcements %T are not this owner's", announcements)
	}
	inner, ok := facade.service.(*service)
	if !ok {
		return nil, fmt.Errorf("surveys: announcement service %T is not this owner's", facade.service)
	}
	records, ok := source.(surveyStoreSource)
	if !ok || records.SurveyRecords() == nil {
		return nil, fmt.Errorf("surveys: announcement source %T keeps no surveys", source)
	}
	return announcementsWithSurveys{
		ParentAnnouncementCapability: announcements, ParentAnnouncementRetention: facade,
		SurveyCapability: newSurveyService(inner, records.SurveyRecords(), staff),
	}, nil
}

// newSurveyService builds the surveys over the announcement service that
// publishes the carrier of a parent survey.
func newSurveyService(announcements *service, store SurveyStore, staff SurveyStaff) *surveyService {
	return &surveyService{
		surveyReads: store, announcements: announcements, store: store, staff: staff,
		logger: announcements.logger.With("service", "survey"), now: time.Now, shuffle: shuffleTexts,
	}
}

// GetSurvey reads one survey of the school.
func (s *surveyService) GetSurvey(ctx context.Context, id int64) (*communication.Survey, error) {
	return s.find(ctx, id, false)
}

func (s *surveyService) find(ctx context.Context, id int64, lock bool) (*communication.Survey, error) {
	survey, err := s.store.FindSurvey(ctx, id, lock)
	if err != nil {
		return nil, err
	}
	if survey == nil {
		return nil, communication.ErrSurveyNotFound
	}
	return survey, nil
}

func (s *surveyService) CreateSurvey(ctx context.Context, createdBy int64, in communication.SurveyInput) (*communication.Survey, error) {
	normalized, err := normalizeSurveyInput(in, s.now())
	if err != nil {
		return nil, err
	}
	var author *int64
	if createdBy > 0 {
		author = &createdBy
	}
	survey, err := s.store.CreateSurvey(ctx, normalized, author)
	if err != nil {
		return nil, err
	}
	s.logger.Info("survey created",
		slog.Int64("survey_id", survey.ID),
		slog.String("audience", survey.Audience),
		slog.Bool("anonymous", survey.Anonymous),
		slog.Int("question_count", len(survey.Questions)),
	)
	return survey, nil
}

func (s *surveyService) UpdateSurvey(ctx context.Context, id int64, in communication.SurveyInput) (*communication.Survey, error) {
	if _, err := s.find(ctx, id, true); err != nil {
		return nil, err
	}
	normalized, err := normalizeSurveyInput(in, s.now())
	if err != nil {
		return nil, err
	}
	replaced, err := s.store.ReplaceDraft(ctx, id, normalized)
	if err != nil {
		return nil, err
	}
	if !replaced {
		return nil, communication.ErrSurveyNotDraft
	}
	return s.find(ctx, id, false)
}

// DeleteSurvey removes a survey with every answer and, for a parent survey,
// its carrier announcement with the mail and pushes still pending.
func (s *surveyService) DeleteSurvey(ctx context.Context, id int64) error {
	survey, err := s.find(ctx, id, true)
	if err != nil {
		return err
	}
	if survey.AnnouncementID != nil {
		if err := s.announcements.deleteSurveyCarrier(ctx, *survey.AnnouncementID); err != nil {
			return err
		}
	}
	if err := s.store.DeleteSurvey(ctx, id); err != nil {
		return err
	}
	s.logger.Info("survey deleted", slog.Int64("survey_id", id))
	return nil
}

// PublishSurvey opens a draft for answers. From here on nothing but its end
// changes: the people asked were told who will see their answers.
func (s *surveyService) PublishSurvey(ctx context.Context, publishedBy, id int64) (*communication.Survey, error) {
	survey, err := s.find(ctx, id, true)
	if err != nil {
		return nil, err
	}
	if survey.PublishedAt != nil {
		return nil, communication.ErrSurveyNotDraft
	}
	now := s.now()
	if !survey.ClosesAt.After(now) {
		return nil, surveyInvalid("closes_at has passed")
	}
	if len(survey.Questions) == 0 {
		return nil, surveyInvalid("a survey needs at least one question")
	}
	var announcementID *int64
	if survey.Audience == communication.SurveyAudienceParents {
		carrier, err := s.announcements.publishSurveyCarrier(ctx, survey, publishedBy)
		if err != nil {
			return nil, err
		}
		announcementID = &carrier
	}
	published, err := s.store.MarkPublished(ctx, id, now, announcementID)
	if err != nil {
		return nil, err
	}
	if !published {
		return nil, communication.ErrSurveyNotDraft
	}
	if survey.Audience == communication.SurveyAudienceStaff {
		if err := s.notifyStaff(ctx, survey, "staff-survey:%d", false, nil); err != nil {
			return nil, err
		}
	}
	s.logger.Info("survey published",
		slog.Int64("survey_id", id),
		slog.String("audience", survey.Audience),
		slog.Bool("anonymous", survey.Anonymous),
	)
	return s.find(ctx, id, false)
}

// CloseSurvey ends the answering now.
func (s *surveyService) CloseSurvey(ctx context.Context, id int64) (*communication.Survey, error) {
	survey, err := s.find(ctx, id, true)
	if err != nil {
		return nil, err
	}
	now := s.now()
	if survey.Status(now) != communication.SurveyStatusOpen {
		return nil, communication.ErrSurveyNotOpen
	}
	if err := s.store.SetClosesAt(ctx, id, now); err != nil {
		return nil, err
	}
	s.logger.Info("survey closed", slog.Int64("survey_id", id))
	return s.find(ctx, id, false)
}

// RemindSurvey nudges everyone of the audience who has not taken part. The
// school learns how many were reminded, never who.
func (s *surveyService) RemindSurvey(ctx context.Context, id int64) (int, error) {
	survey, err := s.find(ctx, id, false)
	if err != nil {
		return 0, err
	}
	if survey.Status(s.now()) != communication.SurveyStatusOpen {
		return 0, communication.ErrSurveyNotOpen
	}
	participants, err := s.store.ParticipantAccounts(ctx, id)
	if err != nil {
		return 0, err
	}
	participated := make(map[int64]bool, len(participants))
	for _, accountID := range participants {
		participated[accountID] = true
	}
	if survey.Audience == communication.SurveyAudienceParents {
		return s.announcements.remindSurveyGuardians(ctx, survey, participated)
	}
	reminded := 0
	key := fmt.Sprintf("staff-survey-reminder:%d:%d:", id, s.now().UnixNano()) + "%d"
	err = s.notifyStaff(ctx, survey, key, true, func(accountIDs []int64) []int64 {
		remaining := accountIDs[:0:0]
		for _, accountID := range accountIDs {
			if !participated[accountID] {
				remaining = append(remaining, accountID)
			}
		}
		reminded = len(remaining)
		return remaining
	})
	return reminded, err
}

func (s *surveyService) SurveyResults(ctx context.Context, id int64) (*communication.SurveyResults, error) {
	survey, err := s.store.FindSurvey(ctx, id, false)
	if err != nil {
		return nil, err
	}
	if survey == nil {
		return nil, communication.ErrSurveyNotFound
	}
	tally, err := s.store.Tally(ctx, id)
	if err != nil {
		return nil, err
	}
	return buildSurveyResults(survey, tally, s.now(), s.shuffle), nil
}

// SurveyRespondents lists a named survey's answers per person. An anonymous
// survey has none to give, whatever the caller may hold.
func (s *surveyService) SurveyRespondents(ctx context.Context, id int64) ([]communication.SurveyRespondent, error) {
	survey, err := s.find(ctx, id, false)
	if err != nil {
		return nil, err
	}
	if survey.Anonymous {
		return nil, communication.ErrSurveyAnonymous
	}
	responses, err := s.store.NamedResponses(ctx, id)
	if err != nil {
		return nil, err
	}
	names, err := s.respondentNames(ctx, survey)
	if err != nil {
		return nil, err
	}
	return namedRespondents(survey, responses, names), nil
}

func (s *surveyService) respondentNames(ctx context.Context, survey *communication.Survey) (map[int64]string, error) {
	names := map[int64]string{}
	if survey.Audience == communication.SurveyAudienceStaff {
		staff, err := s.staffAudience(ctx)
		if err != nil {
			return nil, err
		}
		for _, member := range staff {
			names[member.AccountID] = member.Name
		}
		return names, nil
	}
	if survey.AnnouncementID == nil {
		return names, nil
	}
	recipients, err := s.announcements.repo.ResolveDeliveryRecipients(ctx, survey.TenantID, *survey.AnnouncementID)
	if err != nil {
		return nil, fmt.Errorf("survey: resolve respondent names: %w", err)
	}
	for _, r := range recipients {
		if r.AccountID != nil {
			names[*r.AccountID] = strings.TrimSpace(r.FirstName + " " + r.LastName)
		}
	}
	return names, nil
}

func (s *surveyService) staffAudience(ctx context.Context) ([]*communication.AddressableStaffRecord, error) {
	if s.staff == nil {
		return nil, errors.New("survey: staff audience is not wired")
	}
	staff, err := s.staff.ListAddressableStaff(ctx, 0)
	if err != nil {
		return nil, fmt.Errorf("survey: list staff audience: %w", err)
	}
	out := make([]*communication.AddressableStaffRecord, 0, len(staff))
	for _, member := range staff {
		if member != nil && member.RoleKind != surveyLehrkraftRoleKind {
			out = append(out, member)
		}
	}
	return out, nil
}
