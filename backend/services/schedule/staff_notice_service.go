// Tagesinformationen (#2180) — Geschäftslogik der Hinweise fürs Team:
// interner Hinweise der Leitung an das Team, die an bestimmten Tagen gelten.
//
// Wichtig für die Einordnung: hier entsteht KEINE zweite Recurrence-Engine. Ein
// Hinweis wird nie in Tageszeilen materialisiert, er wird beim Lesen gegen das
// Datum geprüft. Das Vokabular (Wochentage, Wochenmuster, Gültigkeitszeitraum)
// und die Auswertung des Wochenmusters kommen aus dem Stundenplan
// (schedule.ShouldMaterializeWeekPattern), damit "Woche A" hier dasselbe heißt
// wie dort.
package schedule

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

// ErrStaffNoticeNotFound meldet einen unbekannten oder fremden Hinweis.
var ErrStaffNoticeNotFound = errors.New("staffnotice: notice not found")

// ErrStaffNoticeInvalid meldet fachlich unzulässige Eingaben.
var ErrStaffNoticeInvalid = errors.New("staffnotice: invalid notice")

// StaffNoticeInput ist die Schreibform eines Hinweises.
type StaffNoticeInput struct {
	Title                   string
	Body                    string
	Priority                string
	ValidFrom               timezone.Date
	ValidUntil              *timezone.Date
	Weekdays                []int16
	WeekPattern             int
	RequiresAcknowledgement bool
	Active                  bool
}

// StaffNoticeService ist der Vertrag der Tagesinformationen.
type StaffNoticeService interface {
	// List gibt alle Hinweise des Mandanten zurück (Leitungssicht), jeweils mit
	// der Zahl der Kenntnisnahmen.
	List(ctx context.Context, accountID int64, includeInactive bool) ([]*usersModels.StaffNoticeView, error)
	// Today gibt die Hinweise zurück, die an diesem Kalendertag gelten — die
	// Sicht des Teams auf der Startseite.
	Today(ctx context.Context, accountID int64, date timezone.Date) ([]*usersModels.StaffNoticeView, error)
	Get(ctx context.Context, id int64) (*usersModels.StaffNotice, error)
	Create(ctx context.Context, createdBy int64, in StaffNoticeInput) (*usersModels.StaffNotice, error)
	Update(ctx context.Context, id int64, in StaffNoticeInput) (*usersModels.StaffNotice, error)
	Delete(ctx context.Context, id int64) error
	// Acknowledge nimmt die Kenntnisnahme einer Person entgegen.
	Acknowledge(ctx context.Context, id, accountID int64) error
}

// StaffNoticePeriodLookup ist der Ausschnitt des Kalenderzeitraum-Repositories, den die
// Auflösung des Wochenmusters braucht. Bewusst hier deklariert und nicht das
// volle Repository verlangt: der Dienst liest Zeiträume, er verwaltet keine.
type StaffNoticePeriodLookup interface {
	FindActiveByTenantID(ctx context.Context) ([]*scheduleModels.CalendarPeriod, error)
}

// StaffNoticeServiceConfig ist das Abhängigkeitsbündel. Periods ist optional: ohne
// Kalenderzeitraum lässt sich kein Wochenmuster auflösen, dann gilt ein Hinweis
// in jeder Woche (dieselbe Richtung wie ShouldMaterializeWeekPattern).
type StaffNoticeServiceConfig struct {
	Repo        usersModels.StaffNoticeRepository
	Periods     StaffNoticePeriodLookup
	Logger      *slog.Logger
	CurrentDate func() timezone.Date
}

type staffNoticeService struct {
	repo        usersModels.StaffNoticeRepository
	periods     StaffNoticePeriodLookup
	logger      *slog.Logger
	currentDate func() timezone.Date
}

// NewStaffNoticeService verdrahtet den Dienst.
func NewStaffNoticeService(cfg StaffNoticeServiceConfig) StaffNoticeService {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	currentDate := cfg.CurrentDate
	if currentDate == nil {
		currentDate = timezone.TodayDate
	}
	return &staffNoticeService{repo: cfg.Repo, periods: cfg.Periods, logger: logger, currentDate: currentDate}
}

func (s *staffNoticeService) Get(ctx context.Context, id int64) (*usersModels.StaffNotice, error) {
	notice, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("staffnotice: get: %w", err)
	}
	if notice == nil {
		return nil, ErrStaffNoticeNotFound
	}
	return notice, nil
}

func (s *staffNoticeService) List(ctx context.Context, accountID int64, includeInactive bool) ([]*usersModels.StaffNoticeView, error) {
	rows, err := s.repo.List(ctx, includeInactive)
	if err != nil {
		return nil, fmt.Errorf("staffnotice: list: %w", err)
	}
	return s.decorate(ctx, accountID, rows, true)
}

func (s *staffNoticeService) Today(ctx context.Context, accountID int64, date timezone.Date) ([]*usersModels.StaffNoticeView, error) {
	rows, err := s.repo.ListValidOn(ctx, date)
	if err != nil {
		return nil, fmt.Errorf("staffnotice: today: %w", err)
	}

	matching := make([]*usersModels.StaffNotice, 0, len(rows))
	for _, notice := range rows {
		if !notice.AppliesOn(date) {
			continue
		}
		matching = append(matching, notice)
	}

	matching, err = s.filterByWeekPattern(ctx, matching, date)
	if err != nil {
		return nil, err
	}
	return s.decorate(ctx, accountID, matching, false)
}

// filterByWeekPattern wirft die Hinweise raus, deren Woche heute nicht dran
// ist. Die Kalenderzeiträume werden nur geladen, wenn überhaupt ein Hinweis ein
// Muster trägt — der Normalfall "gilt jede Woche" soll die Startseite keine
// zusätzliche Abfrage kosten.
func (s *staffNoticeService) filterByWeekPattern(
	ctx context.Context,
	notices []*usersModels.StaffNotice,
	date timezone.Date,
) ([]*usersModels.StaffNotice, error) {
	needsPeriod := false
	for _, notice := range notices {
		if notice.WeekPattern != scheduleModels.WeekPatternEvery {
			needsPeriod = true
			break
		}
	}
	if !needsPeriod || s.periods == nil {
		return notices, nil
	}

	period, err := s.periodFor(ctx, date)
	if err != nil {
		return nil, err
	}

	kept := make([]*usersModels.StaffNotice, 0, len(notices))
	for _, notice := range notices {
		if ShouldMaterializeWeekPattern(notice.WeekPattern, date, period) {
			kept = append(kept, notice)
		}
	}
	return kept, nil
}

// periodFor sucht das aktive Schuljahr, das den Tag enthält und einen
// Wochenzyklus führt. Für Tagesinformationen ist das Schuljahr der eindeutige
// Träger von Woche A/B: Ferien, Halbjahre und eigene Zeiträume dürfen sich
// damit überschneiden, ohne die Wiederholung zu verändern. Ohne Treffer nil —
// ShouldMaterializeWeekPattern lässt den Hinweis dann durch, statt ihn stumm
// verschwinden zu lassen.
func (s *staffNoticeService) periodFor(ctx context.Context, date timezone.Date) (*scheduleModels.CalendarPeriod, error) {
	periods, err := s.periods.FindActiveByTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("staffnotice: load calendar periods: %w", err)
	}
	for _, period := range periods {
		if period.PeriodType != scheduleModels.PeriodTypeSchoolYear {
			continue
		}
		if period.WeekCycleLength <= 1 || period.WeekCycleAnchor == nil {
			continue
		}
		if date.Before(timezone.Date(period.StartDate)) || date.After(timezone.Date(period.EndDate)) {
			continue
		}
		return period, nil
	}
	return nil, nil
}

// decorate hängt an jede Zeile die eigene Kenntnisnahme und für die Leitung
// optional die Gesamtzahl der Kenntnisnahmen — jeweils gebündelt, ohne N+1.
func (s *staffNoticeService) decorate(
	ctx context.Context,
	accountID int64,
	notices []*usersModels.StaffNotice,
	includeAcknowledgedCounts bool,
) ([]*usersModels.StaffNoticeView, error) {
	views := make([]*usersModels.StaffNoticeView, 0, len(notices))
	if len(notices) == 0 {
		return views, nil
	}

	ids := make([]int64, 0, len(notices))
	for _, notice := range notices {
		ids = append(ids, notice.ID)
	}

	own, err := s.repo.AcknowledgedAtFor(ctx, accountID, ids)
	if err != nil {
		return nil, fmt.Errorf("staffnotice: own acknowledgements: %w", err)
	}
	counts := map[int64]int{}
	if includeAcknowledgedCounts {
		counts, err = s.repo.AcknowledgedCounts(ctx, ids)
		if err != nil {
			return nil, fmt.Errorf("staffnotice: acknowledgement counts: %w", err)
		}
	}

	for _, notice := range notices {
		view := &usersModels.StaffNoticeView{
			StaffNotice:       notice,
			AcknowledgedCount: counts[notice.ID],
		}
		if at, ok := own[notice.ID]; ok {
			stamped := at
			view.AcknowledgedAt = &stamped
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *staffNoticeService) Create(ctx context.Context, createdBy int64, in StaffNoticeInput) (*usersModels.StaffNotice, error) {
	notice, err := s.apply(&usersModels.StaffNotice{CreatedBy: createdBy}, in)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, notice); err != nil {
		return nil, fmt.Errorf("staffnotice: create: %w", err)
	}
	s.logger.Info("staff_notice_created",
		slog.Int64("notice_id", notice.ID),
		slog.Int64("created_by", createdBy),
		slog.String("priority", notice.Priority),
	)
	return notice, nil
}

func (s *staffNoticeService) Update(ctx context.Context, id int64, in StaffNoticeInput) (*usersModels.StaffNotice, error) {
	existing, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	notice, err := s.apply(existing, in)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, notice); err != nil {
		return nil, fmt.Errorf("staffnotice: update: %w", err)
	}
	s.logger.Info("staff_notice_updated", slog.Int64("notice_id", notice.ID))
	return notice, nil
}

func (s *staffNoticeService) Delete(ctx context.Context, id int64) error {
	if _, err := s.Get(ctx, id); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("staffnotice: delete: %w", err)
	}
	s.logger.Info("staff_notice_deleted", slog.Int64("notice_id", id))
	return nil
}

func (s *staffNoticeService) Acknowledge(ctx context.Context, id, accountID int64) error {
	notice, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if !notice.RequiresAcknowledgement {
		// Ein Hinweis ohne angeforderte Kenntnisnahme hat keine zu speichern.
		// Das ist kein Fehler der Person, sondern eine veraltete Ansicht.
		return fmt.Errorf("%w: notice does not ask for acknowledgement", ErrStaffNoticeInvalid)
	}
	today := s.currentDate()
	if !notice.AppliesOn(today) {
		return fmt.Errorf("%w: notice does not apply today", ErrStaffNoticeInvalid)
	}
	matching, err := s.filterByWeekPattern(ctx, []*usersModels.StaffNotice{notice}, today)
	if err != nil {
		return err
	}
	if len(matching) == 0 {
		return fmt.Errorf("%w: notice does not apply today", ErrStaffNoticeInvalid)
	}
	if err := s.repo.Acknowledge(ctx, id, accountID); err != nil {
		return fmt.Errorf("staffnotice: acknowledge: %w", err)
	}
	return nil
}

// apply überträgt die Eingabe auf die Zeile und prüft sie. Der Zuschnitt der
// Wochentage passiert hier und nicht im Modell: doppelte Einträge sind eine
// Eingabefrage, keine Eigenschaft des Hinweises.
func (s *staffNoticeService) apply(notice *usersModels.StaffNotice, in StaffNoticeInput) (*usersModels.StaffNotice, error) {
	notice.Title = strings.TrimSpace(in.Title)
	notice.Body = strings.TrimSpace(in.Body)
	notice.Priority = in.Priority
	if notice.Priority == "" {
		notice.Priority = usersModels.StaffNoticePriorityInfo
	}
	notice.ValidFrom = in.ValidFrom
	notice.ValidUntil = in.ValidUntil
	notice.Weekdays = normalizeNoticeWeekdays(in.Weekdays)
	notice.WeekPattern = in.WeekPattern
	notice.RequiresAcknowledgement = in.RequiresAcknowledgement
	notice.Active = in.Active

	if notice.ValidFrom.IsZero() {
		return nil, fmt.Errorf("%w: valid from is required", ErrStaffNoticeInvalid)
	}
	if err := notice.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrStaffNoticeInvalid, err.Error())
	}
	return notice, nil
}

// normalizeNoticeWeekdays sortiert aufsteigend und entfernt Doppelte. Unzulässige
// Werte bleiben absichtlich stehen, damit Validate sie ablehnt, statt sie
// stillschweigend zu schlucken. Alle sieben Tage bedeuten dasselbe wie "keine
// Angabe", werden aber nicht zusammengefasst: die Leitung soll ihre Auswahl
// wiederfinden.
func normalizeNoticeWeekdays(in []int16) []int16 {
	seen := make(map[int16]bool, len(in))
	out := make([]int16, 0, len(in))
	for _, day := range in {
		if seen[day] {
			continue
		}
		seen[day] = true
		out = append(out, day)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
