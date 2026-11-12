package holidayclosurenotification

import (
	"context"
	"errors"
	"hash/fnv"
	"log/slog"
	"maps"
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
	Mails     ports.Mails
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
		dependencies.Schools == nil || dependencies.Units == nil || dependencies.Mails == nil {
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

func sourceRanges(ranges ...*schoolcalendar.HolidayClosureRange) []enrollment.HolidayClosureRange {
	var result []enrollment.HolidayClosureRange
	for _, r := range ranges {
		if r != nil {
			result = append(result, enrollment.HolidayClosureRange{From: r.StartsOn, To: r.EndsOn})
		}
	}
	return result
}

// plans previews, keyed by proposal, the notices of one Träger that wait
// for a click, in the caller's Träger transaction of that Träger and in
// the same statements for any number of notices. A school that left the
// Träger has an empty plan.
func (s *Service) plans(ctx context.Context, organizationID int64, notices []schoolcalendar.HolidayClosureNotice) (map[int64]enrollment.HolidayClosureImpact, error) {
	var waiting []schoolcalendar.HolidayClosureNotice
	var operations []enrollment.HolidayClosureOperation
	for _, notice := range notices {
		if notice.OrganizationID == organizationID && notice.Status == schoolcalendar.HolidayClosureSuggested {
			waiting = append(waiting, notice)
			planned := operation(notice.SchoolID, notice.OperationID, notice.SpanFrom, notice.SpanTo)
			planned.PreserveSpan = notice.ConfirmedAt != nil
			planned.Ranges = sourceRanges(notice.Previous, notice.Current)
			planned.ConstrainRanges = true
			operations = append(operations, planned)
		}
	}
	result := make(map[int64]enrollment.HolidayClosureImpact, len(waiting))
	if len(waiting) == 0 {
		return result, nil
	}
	impacts, err := s.Closures.PlanHolidayClosureOperations(ctx, organizationID, operations)
	if err != nil {
		return nil, err
	}
	for i, notice := range waiting {
		impact := impacts[i]
		if impact.Weeks == nil {
			impact.Weeks = []enrollment.HolidayClosureWeek{}
		}
		result[notice.ProposalID] = impact
	}
	return result, nil
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
		if notice.Status == schoolcalendar.HolidayClosureSuggested && families[notice.ProposalID].Families == 0 {
			continue
		}
		item := view(notice)
		item.Families = families[notice.ProposalID].Families
		item.Token = families[notice.ProposalID].Token
		result = append(result, item)
	}
	return result, nil
}

// waitingFamilies counts, per notice that waits for a click, the families a
// click would tell, in a Träger transaction of each Träger of its own.
func (s *Service) waitingFamilies(ctx context.Context, notices []schoolcalendar.HolidayClosureNotice) (map[int64]enrollment.HolidayClosureImpact, error) {
	families := make(map[int64]enrollment.HolidayClosureImpact, len(notices))
	for _, organizationID := range organizationsOf(notices, true) {
		if err := s.Units.WithinTraeger(ctx, organizationID, func(ctx context.Context) error {
			planned, err := s.plans(ctx, organizationID, notices)
			maps.Copy(families, planned)
			return err
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
func (s *Service) NotifyFromSchool(ctx context.Context, schoolID, accountID, noticeID, revision int64, token string) (Notice, error) {
	return s.confirm(ctx, schoolID, noticeID, revision, schoolcalendar.HolidayClosureActor{
		AccountID: accountID, Channel: schoolcalendar.HolidayClosureChannelSchool, ImpactToken: token,
	})
}

// NotifyFromTraeger checks in a Träger transaction that the notice belongs
// to a current school of the Träger, then confirms it in the school's own
// transaction: never a school transaction inside the Träger's.
func (s *Service) NotifyFromTraeger(ctx context.Context, organizationID, accountID, noticeID, revision int64, token string) (Notice, error) {
	notice, err := s.traegerNotice(ctx, organizationID, noticeID)
	if err != nil {
		return Notice{}, err
	}
	return s.confirm(ctx, notice.SchoolID, noticeID, revision, schoolcalendar.HolidayClosureActor{
		AccountID: accountID, Channel: schoolcalendar.HolidayClosureChannelTraeger, OrganizationID: organizationID, ImpactToken: token,
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
	seen, err := s.reread(ctx, schoolID, noticeID)
	if err != nil {
		return Notice{}, err
	}
	if actor.OrganizationID != 0 && seen.OrganizationID != actor.OrganizationID {
		return Notice{}, ErrNoticeNotFound
	}
	if err := s.validateImpact(ctx, seen, actor.ImpactToken); err != nil {
		return Notice{}, err
	}
	var confirmed schoolcalendar.HolidayClosureNotice
	err = s.Units.WithinSchool(ctx, schoolID, func(ctx context.Context) error {
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

// validateImpact compares the displayed cohort before the school confirms.
// Preparation rechecks it in the Träger unit that records the mail.
func (s *Service) validateImpact(ctx context.Context, notice schoolcalendar.HolidayClosureNotice, token string) error {
	if notice.Status != schoolcalendar.HolidayClosureSuggested {
		return nil
	}
	return s.Units.WithinTraeger(ctx, notice.OrganizationID, func(ctx context.Context) error {
		planned, err := s.plans(ctx, notice.OrganizationID, []schoolcalendar.HolidayClosureNotice{notice})
		if err != nil {
			return err
		}
		if token == "" || planned[notice.ProposalID].Token != token {
			return ErrNoticeChanged
		}
		return nil
	})
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
		notices, err := s.Proposals.TraegerHolidayClosureNotices(ctx, organizationID)
		if err != nil {
			return err
		}
		var schools map[int64]string
		if len(notices) > 0 {
			schools, err = s.currentSchools(ctx, organizationID)
			if err != nil {
				return err
			}
		}
		notices = currentNotices(notices, organizationID, schools)
		planned, err := s.plans(ctx, organizationID, notices)
		if err != nil {
			return err
		}
		tokens, err := s.mailRecovery(ctx, organizationID, &pending)
		if err != nil {
			return err
		}
		tokens = append(tokens, appendPendingNotices(&pending, notices, schools, planned)...)
		pending.Token = pendingToken(tokens)
		return nil
	})
	return pending, err
}

// traegerItem is the Büro's view of a notice; one that waits for a click
// carries its plan and is left out when no family would be told.
func traegerItem(notice schoolcalendar.HolidayClosureNotice, schools map[int64]string, planned map[int64]enrollment.HolidayClosureImpact) (TraegerNotice, enrollment.HolidayClosureImpact, bool) {
	item := TraegerNotice{Notice: view(notice), Weeks: []enrollment.HolidayClosureWeek{}}
	item.SchoolName = schools[notice.SchoolID]
	if notice.Status != schoolcalendar.HolidayClosureSuggested {
		return item, enrollment.HolidayClosureImpact{}, true
	}
	impact := planned[notice.ProposalID]
	if impact.Families == 0 {
		return item, impact, false
	}
	item.Families, item.Children, item.Weeks = impact.Families, impact.Children, impact.Weeks
	item.Token = impact.Token
	return item, impact, true
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
// the same command. The list was planned together and its token checked
// just now; each command carries its impact token, which the worker checks
// again before it prepares mail. The notices of one school are confirmed in
// one unit of that school, and the Träger is processed once at the end.
func (s *Service) NotifyPendingHolidayClosures(ctx context.Context, organizationID, accountID int64, token string) (NotifiedPending, error) {
	pending, err := s.PendingHolidayClosures(ctx, organizationID)
	if err != nil {
		return NotifiedPending{}, err
	}
	if pending.Token != token {
		return NotifiedPending{}, ErrNoticeChanged
	}
	bySchool := map[int64][]TraegerNotice{}
	for _, notice := range pending.Notices {
		if notice.Status == string(schoolcalendar.HolidayClosureSuggested) {
			bySchool[notice.SchoolID] = append(bySchool[notice.SchoolID], notice)
		}
	}
	var result NotifiedPending
	result.RetriedMails, err = s.retryPendingMails(ctx, organizationID, pending.mailKeys)
	if err != nil {
		return result, err
	}
	var operations []string
	for _, schoolID := range slices.Sorted(maps.Keys(bySchool)) {
		confirmed, operationIDs, err := s.confirmAtSchool(ctx, organizationID, accountID, schoolID, bySchool[schoolID])
		result.Notices += confirmed.Notices
		operations = append(operations, operationIDs...)
		if err != nil {
			return result, err
		}
	}
	if result.Notices > 0 || result.RetriedMails > 0 {
		if err := s.processTraeger(ctx, organizationID); err != nil {
			s.Logger.Warn("holiday closure notices left for the worker",
				"organization_id", organizationID,
				"error", err.Error(),
			)
		}
	}
	result.Families, err = s.confirmedFamilies(ctx, organizationID, operations)
	return result, err
}

// confirmAtSchool confirms the Büro's notices of one school in one unit of
// that school.
func (s *Service) confirmAtSchool(ctx context.Context, organizationID, accountID, schoolID int64, notices []TraegerNotice) (NotifiedPending, []string, error) {
	var result NotifiedPending
	var operations []string
	err := s.Units.WithinSchool(ctx, schoolID, func(ctx context.Context) error {
		result = NotifiedPending{}
		operations = nil
		confirmations := make([]schoolcalendar.HolidayClosureConfirmation, len(notices))
		for i, notice := range notices {
			confirmations[i] = schoolcalendar.HolidayClosureConfirmation{ProposalID: notice.ID, Revision: notice.Revision, ImpactToken: notice.Token}
		}
		confirmed, err := s.Proposals.ConfirmHolidayClosureNotices(ctx, confirmations, schoolcalendar.HolidayClosureActor{
			AccountID: accountID, Channel: schoolcalendar.HolidayClosureChannelTraeger, OrganizationID: organizationID,
		})
		if err != nil {
			return err
		}
		for _, notice := range confirmed {
			s.Logger.Info("holiday closure notice confirmed",
				"proposal_id", notice.ProposalID,
				"school_id", schoolID,
				"organization_id", notice.OrganizationID,
				"account_id", accountID,
				"channel", schoolcalendar.HolidayClosureChannelTraeger,
				"event_id", notice.EventID,
				"status", string(notice.Status),
			)
			result.Notices++
			operations = append(operations, notice.OperationID)
		}
		return nil
	})
	switch {
	case errors.Is(err, schoolcalendar.ErrHolidayClosureNoticeNotFound):
		return NotifiedPending{}, nil, ErrNoticeNotFound
	case errors.Is(err, schoolcalendar.ErrHolidayClosureNoticeChanged):
		return NotifiedPending{}, nil, ErrNoticeChanged
	case err != nil:
		return NotifiedPending{}, nil, err
	}
	return result, operations, nil
}

// confirmedFamilies counts actual logical mail requests. Overlapping approved
// notices may leave a later operation with no remaining days to tell.
func (s *Service) confirmedFamilies(ctx context.Context, organizationID int64, operations []string) (int, error) {
	if len(operations) == 0 {
		return 0, nil
	}
	count := 0
	err := s.Units.WithinTraeger(ctx, organizationID, func(ctx context.Context) error {
		deliveries, err := s.Closures.HolidayClosureOperationsDelivery(ctx, organizationID, operations)
		if err != nil {
			return err
		}
		for _, delivery := range deliveries {
			count += delivery.Queued + delivery.Sent + delivery.Failed + delivery.Unknown
		}
		return nil
	})
	return count, err
}
