package holidayclosurenotification

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/holidayclosurenotification/ports"
)

// Dependencies are the owners' ports and the units.
type Dependencies struct {
	Proposals ports.Proposals
	Queue     ports.Queue
	Closures  ports.Closures
	Schools   ports.Schools
	Units     ports.Units
	Logger    *slog.Logger
}

// Service is the protocol of the workflow (ADR 0196): which unit of which
// owner runs when.
type Service struct {
	Dependencies
}

var _ Notifications = (*Service)(nil)

// New builds the workflow over the ports the root binds.
func New(dependencies Dependencies) *Service {
	if dependencies.Proposals == nil || dependencies.Queue == nil || dependencies.Closures == nil ||
		dependencies.Schools == nil || dependencies.Units == nil {
		panic("holiday closure notification: every dependency is required")
	}
	if dependencies.Logger == nil {
		dependencies.Logger = slog.Default()
	}
	return &Service{Dependencies: dependencies}
}

// operation is the Enrollment operation of a proposal or command of school;
// Enrollment reads the school's calendar through the worker's read channel
// in the caller's Träger transaction, the same for count, preview and
// notice.
func operation(schoolID int64, operationID string, from, to calendar.Date) enrollment.HolidayClosureOperation {
	return enrollment.HolidayClosureOperation{OperationID: operationID, SchoolID: schoolID, From: from, To: to}
}

// plan previews a notice that waits for a click in the caller's Träger
// transaction of its Träger. A school that left the Träger has none.
func (s *Service) plan(ctx context.Context, notice schoolcalendar.HolidayClosureNotice) (enrollment.HolidayClosureImpact, error) {
	impact, err := s.Closures.PlanHolidayClosureOperation(ctx, notice.OrganizationID,
		operation(notice.SchoolID, notice.OperationID, notice.SpanFrom, notice.SpanTo))
	if errors.Is(err, schoolcalendar.ErrHolidayClosureNoticeNotFound) {
		return enrollment.HolidayClosureImpact{Weeks: []enrollment.HolidayClosureWeek{}}, nil
	}
	return impact, err
}

func view(notice schoolcalendar.HolidayClosureNotice) Notice {
	result := Notice{
		ID: notice.ProposalID, SchoolID: notice.SchoolID, ClosingDayID: notice.ClosingDayID, Reason: notice.Reason,
		Revision: notice.Revision, Status: string(notice.Status), Changed: notice.Changed, CanNotify: notice.CanNotify(),
		ConfirmedAt: notice.ConfirmedAt, UpdatedAt: notice.UpdatedAt,
	}
	if notice.Previous != nil {
		result.Previous = &Range{StartDate: notice.Previous.StartsOn, EndDate: notice.Previous.EndsOn}
	}
	if notice.Current != nil {
		result.Current = &Range{StartDate: notice.Current.StartsOn, EndDate: notice.Current.EndsOn}
	}
	return result
}

// SchoolNotices lists the school's notices. The school reads its proposals
// in its own transaction; the count of a notice that waits for a click
// comes from Enrollment in a separate Träger transaction, as a number only.
// A proposal no family would be told about is no proposal for the school.
func (s *Service) SchoolNotices(ctx context.Context, schoolID int64) ([]Notice, error) {
	var notices []schoolcalendar.HolidayClosureNotice
	if err := s.Units.WithinSchool(ctx, schoolID, func(ctx context.Context) error {
		var err error
		notices, err = s.Proposals.SchoolHolidayClosureNotices(ctx)
		return err
	}); err != nil {
		return nil, err
	}
	families, err := s.waitingFamilies(ctx, notices)
	if err != nil {
		return nil, err
	}
	result := make([]Notice, 0, len(notices))
	for _, notice := range notices {
		if notice.Status == schoolcalendar.HolidayClosureSuggested && families[notice.ProposalID] == 0 {
			continue
		}
		item := view(notice)
		item.Families = families[notice.ProposalID]
		result = append(result, item)
	}
	return result, nil
}

// waitingFamilies counts, per notice that waits for a click, the families a
// click would tell, in a Träger transaction of each Träger of its own.
func (s *Service) waitingFamilies(ctx context.Context, notices []schoolcalendar.HolidayClosureNotice) (map[int64]int, error) {
	families := make(map[int64]int, len(notices))
	for _, organizationID := range organizationsOf(notices, true) {
		if err := s.Units.WithinTraeger(ctx, organizationID, func(ctx context.Context) error {
			for _, notice := range notices {
				if notice.OrganizationID != organizationID || notice.Status != schoolcalendar.HolidayClosureSuggested {
					continue
				}
				impact, err := s.plan(ctx, notice)
				if err != nil {
					return err
				}
				families[notice.ProposalID] = impact.Families
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}
	return families, nil
}

// organizationsOf names the Träger of the notices that wait for a click.
func organizationsOf(notices []schoolcalendar.HolidayClosureNotice, suggestedOnly bool) []int64 {
	var ids []int64
	for _, notice := range notices {
		if !suggestedOnly || notice.Status == schoolcalendar.HolidayClosureSuggested {
			ids = append(ids, notice.OrganizationID)
		}
	}
	return slices.Compact(slices.Sorted(slices.Values(ids)))
}

// NotifyFromSchool confirms the notice in the school's transaction and then
// processes the Träger's due commands: a failure there leaves the
// committed command for the worker.
func (s *Service) NotifyFromSchool(ctx context.Context, schoolID, accountID, noticeID, revision int64) (Notice, error) {
	return s.confirm(ctx, schoolID, noticeID, revision, schoolcalendar.HolidayClosureActor{
		AccountID: accountID, Channel: schoolcalendar.HolidayClosureChannelSchool,
	})
}

// NotifyFromTraeger checks in a Träger transaction that the notice belongs
// to a current school of the Träger, then confirms it in the school's own
// transaction: never a school transaction inside the Träger's.
func (s *Service) NotifyFromTraeger(ctx context.Context, organizationID, accountID, noticeID, revision int64) (Notice, error) {
	notice, err := s.traegerNotice(ctx, organizationID, noticeID)
	if err != nil {
		return Notice{}, err
	}
	return s.confirm(ctx, notice.SchoolID, noticeID, revision, schoolcalendar.HolidayClosureActor{
		AccountID: accountID, Channel: schoolcalendar.HolidayClosureChannelTraeger, OrganizationID: organizationID,
	})
}

func (s *Service) traegerNotice(ctx context.Context, organizationID, noticeID int64) (schoolcalendar.HolidayClosureNotice, error) {
	var found *schoolcalendar.HolidayClosureNotice
	if err := s.Units.WithinTraeger(ctx, organizationID, func(ctx context.Context) error {
		notices, err := s.traegerNotices(ctx, organizationID)
		if err != nil {
			return err
		}
		for i := range notices {
			if notices[i].ProposalID == noticeID {
				found = &notices[i]
			}
		}
		return nil
	}); err != nil {
		return schoolcalendar.HolidayClosureNotice{}, err
	}
	if found == nil {
		return schoolcalendar.HolidayClosureNotice{}, ErrNoticeNotFound
	}
	return *found, nil
}

// traegerNotices lists the Träger's notices of its current schools only:
// row security limits them already, Organisation & Tenancy decides the
// membership once more, below it. The school names come along.
func (s *Service) traegerNotices(ctx context.Context, organizationID int64) ([]schoolcalendar.HolidayClosureNotice, error) {
	notices, err := s.Proposals.TraegerHolidayClosureNotices(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	schools, err := s.currentSchools(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	return currentNotices(notices, organizationID, schools), nil
}

// currentNotices keeps the notices of the Träger's current schools.
func currentNotices(notices []schoolcalendar.HolidayClosureNotice, organizationID int64, schools map[int64]string) []schoolcalendar.HolidayClosureNotice {
	return slices.DeleteFunc(notices, func(notice schoolcalendar.HolidayClosureNotice) bool {
		_, current := schools[notice.SchoolID]
		return !current || notice.OrganizationID != organizationID
	})
}

// currentSchools names the Träger's undeleted schools by ID.
func (s *Service) currentSchools(ctx context.Context, organizationID int64) (map[int64]string, error) {
	schools, err := s.Schools.ListSchoolsByOrganization(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(schools))
	for _, school := range schools {
		if school.OrganizationID == organizationID && !school.IsDeleted() {
			names[school.ID] = school.Name
		}
	}
	return names, nil
}

func (s *Service) confirm(ctx context.Context, schoolID, noticeID, revision int64, actor schoolcalendar.HolidayClosureActor) (Notice, error) {
	var confirmed schoolcalendar.HolidayClosureNotice
	err := s.Units.WithinSchool(ctx, schoolID, func(ctx context.Context) error {
		var err error
		confirmed, err = s.Proposals.ConfirmHolidayClosureNotice(ctx, noticeID, revision, actor)
		return err
	})
	switch {
	case errors.Is(err, schoolcalendar.ErrHolidayClosureNoticeNotFound):
		return Notice{}, ErrNoticeNotFound
	case errors.Is(err, schoolcalendar.ErrHolidayClosureNoticeChanged):
		return Notice{}, ErrNoticeChanged
	case err != nil:
		return Notice{}, err
	}
	s.Logger.Info("holiday closure notice confirmed",
		"proposal_id", noticeID,
		"school_id", schoolID,
		"organization_id", confirmed.OrganizationID,
		"account_id", actor.AccountID,
		"channel", actor.Channel,
		"event_id", confirmed.EventID,
		"status", string(confirmed.Status),
	)
	// The command is committed; processing it now only spares the parents
	// the worker's wait. A failure stays on the command, which the worker
	// takes up, so the click answers with the committed state.
	if err := s.processTraeger(ctx, confirmed.OrganizationID); err != nil {
		s.Logger.Warn("holiday closure notice left for the worker",
			"organization_id", confirmed.OrganizationID,
			"event_id", confirmed.EventID,
			"error", err.Error(),
		)
	}
	if refreshed, err := s.reread(ctx, schoolID, noticeID); err == nil {
		confirmed = refreshed
	}
	return view(confirmed), nil
}

func (s *Service) reread(ctx context.Context, schoolID, noticeID int64) (schoolcalendar.HolidayClosureNotice, error) {
	var found *schoolcalendar.HolidayClosureNotice
	err := s.Units.WithinSchool(ctx, schoolID, func(ctx context.Context) error {
		notices, err := s.Proposals.SchoolHolidayClosureNotices(ctx)
		for i := range notices {
			if notices[i].ProposalID == noticeID {
				found = &notices[i]
			}
		}
		return err
	})
	if err != nil {
		return schoolcalendar.HolidayClosureNotice{}, err
	}
	if found == nil {
		return schoolcalendar.HolidayClosureNotice{}, ErrNoticeNotFound
	}
	return *found, nil
}

// PendingHolidayClosures lists the notices of the Träger's schools with the
// affected bookings of every notice that waits for a click.
func (s *Service) PendingHolidayClosures(ctx context.Context, organizationID int64) (Pending, error) {
	pending := Pending{
		HolidayClosureImpact: enrollment.HolidayClosureImpact{Weeks: []enrollment.HolidayClosureWeek{}, KeepsContribution: enrollment.HolidayClosureKeepsContribution},
		Notices:              []TraegerNotice{},
	}
	err := s.Units.WithinTraeger(ctx, organizationID, func(ctx context.Context) error {
		schools, err := s.currentSchools(ctx, organizationID)
		if err != nil {
			return err
		}
		notices, err := s.Proposals.TraegerHolidayClosureNotices(ctx, organizationID)
		if err != nil {
			return err
		}
		notices = currentNotices(notices, organizationID, schools)
		var tokens []string
		for _, notice := range notices {
			item, impact, keep, err := s.traegerItem(ctx, notice, schools)
			if err != nil {
				return err
			}
			if !keep {
				continue
			}
			if notice.Status == schoolcalendar.HolidayClosureSuggested {
				pending.Weeks = append(pending.Weeks, impact.Weeks...)
				pending.Children += impact.Children
				pending.Families += impact.Families
				tokens = append(tokens, fmt.Sprintf("%d:%d:%s", notice.ProposalID, notice.Revision, impact.Token))
			}
			pending.Notices = append(pending.Notices, item)
		}
		pending.Token = pendingToken(tokens)
		return nil
	})
	return pending, err
}

// traegerItem is the Büro's view of a notice; one that waits for a click
// carries its plan and is left out when no family would be told.
func (s *Service) traegerItem(ctx context.Context, notice schoolcalendar.HolidayClosureNotice, schools map[int64]string) (TraegerNotice, enrollment.HolidayClosureImpact, bool, error) {
	item := TraegerNotice{Notice: view(notice), Weeks: []enrollment.HolidayClosureWeek{}}
	item.SchoolName = schools[notice.SchoolID]
	if notice.Status != schoolcalendar.HolidayClosureSuggested {
		return item, enrollment.HolidayClosureImpact{}, true, nil
	}
	impact, err := s.plan(ctx, notice)
	if err != nil || impact.Families == 0 {
		return item, impact, false, err
	}
	item.Families, item.Children, item.Weeks = impact.Families, impact.Children, impact.Weeks
	return item, impact, true, nil
}

// pendingToken fingerprints the notices that wait for a click with their
// revisions and affected bookings.
func pendingToken(lines []string) string {
	slices.Sort(lines)
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(strings.Join(lines, "\n")))
	return strconv.FormatUint(hash.Sum64(), 16)
}

// NotifyPendingHolidayClosures confirms every notice of the list the Büro
// saw; each keeps its own operation, so a second click of the school finds
// the same command.
func (s *Service) NotifyPendingHolidayClosures(ctx context.Context, organizationID, accountID int64, token string) (NotifiedPending, error) {
	pending, err := s.PendingHolidayClosures(ctx, organizationID)
	if err != nil {
		return NotifiedPending{}, err
	}
	if pending.Token != token {
		return NotifiedPending{}, ErrNoticeChanged
	}
	var result NotifiedPending
	for _, notice := range pending.Notices {
		if notice.Status != string(schoolcalendar.HolidayClosureSuggested) {
			continue
		}
		if _, err := s.confirm(ctx, notice.SchoolID, notice.ID, notice.Revision, schoolcalendar.HolidayClosureActor{
			AccountID: accountID, Channel: schoolcalendar.HolidayClosureChannelTraeger, OrganizationID: organizationID,
		}); err != nil {
			return result, err
		}
		result.Notices++
		result.Families += notice.Families
	}
	return result, nil
}
