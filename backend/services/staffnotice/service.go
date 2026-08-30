// Package staffnotice ist die Geschäftslogik der Tagesinformationen (#2180):
// interner Hinweise der Leitung an das Team, die an bestimmten Tagen gelten.
//
// Wichtig für die Einordnung: hier entsteht KEINE zweite Recurrence-Engine. Ein
// Hinweis wird nie in Tageszeilen materialisiert, er wird beim Lesen gegen das
// Datum geprüft. Das Vokabular (Wochentage, Wochenmuster, Gültigkeitszeitraum)
// und die Auswertung des Wochenmusters kommen aus dem Stundenplan
// (schedule.ShouldMaterializeWeekPattern), damit "Woche A" hier dasselbe heißt
// wie dort.
package staffnotice

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
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

// ErrNotFound meldet einen unbekannten oder fremden Hinweis.
var ErrNotFound = errors.New("staffnotice: notice not found")

// ErrInvalid meldet fachlich unzulässige Eingaben.
var ErrInvalid = errors.New("staffnotice: invalid notice")

// Input ist die Schreibform eines Hinweises.
type Input struct {
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

// Service ist der Vertrag der Tagesinformationen.
type Service interface {
	// List gibt alle Hinweise des Mandanten zurück (Leitungssicht), jeweils mit
	// der Zahl der Kenntnisnahmen.
	List(ctx context.Context, accountID int64, includeInactive bool) ([]*usersModels.StaffNoticeView, error)
	// Today gibt die Hinweise zurück, die an diesem Kalendertag gelten — die
	// Sicht des Teams auf der Startseite.
	Today(ctx context.Context, accountID int64, date timezone.Date) ([]*usersModels.StaffNoticeView, error)
	Get(ctx context.Context, id int64) (*usersModels.StaffNotice, error)
	Create(ctx context.Context, createdBy int64, in Input) (*usersModels.StaffNotice, error)
	Update(ctx context.Context, id int64, in Input) (*usersModels.StaffNotice, error)
	Delete(ctx context.Context, id int64) error
	// Acknowledge nimmt die Kenntnisnahme einer Person entgegen.
	Acknowledge(ctx context.Context, id, accountID int64) error
}

// PeriodLookup ist der Ausschnitt des Kalenderzeitraum-Repositories, den die
// Auflösung des Wochenmusters braucht. Bewusst hier deklariert und nicht das
// volle Repository verlangt: der Dienst liest Zeiträume, er verwaltet keine.
type PeriodLookup interface {
	FindActiveByTenantID(ctx context.Context) ([]*scheduleModels.CalendarPeriod, error)
}

// ServiceConfig ist das Abhängigkeitsbündel. Periods ist optional: ohne
// Kalenderzeitraum lässt sich kein Wochenmuster auflösen, dann gilt ein Hinweis
// in jeder Woche (dieselbe Richtung wie ShouldMaterializeWeekPattern).
type ServiceConfig struct {
	Repo    usersModels.StaffNoticeRepository
	Periods PeriodLookup
	Logger  *slog.Logger
}

type service struct {
	repo    usersModels.StaffNoticeRepository
	periods PeriodLookup
	logger  *slog.Logger
}

// NewService verdrahtet den Dienst.
func NewService(cfg ServiceConfig) Service {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &service{repo: cfg.Repo, periods: cfg.Periods, logger: logger}
}

func (s *service) Get(ctx context.Context, id int64) (*usersModels.StaffNotice, error) {
	notice, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("staffnotice: get: %w", err)
	}
	if notice == nil {
		return nil, ErrNotFound
	}
	return notice, nil
}

func (s *service) List(ctx context.Context, accountID int64, includeInactive bool) ([]*usersModels.StaffNoticeView, error) {
	rows, err := s.repo.List(ctx, includeInactive)
	if err != nil {
		return nil, fmt.Errorf("staffnotice: list: %w", err)
	}
	return s.decorate(ctx, accountID, rows)
}

func (s *service) Today(ctx context.Context, accountID int64, date timezone.Date) ([]*usersModels.StaffNoticeView, error) {
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
	return s.decorate(ctx, accountID, matching)
}

// filterByWeekPattern wirft die Hinweise raus, deren Woche heute nicht dran
// ist. Die Kalenderzeiträume werden nur geladen, wenn überhaupt ein Hinweis ein
// Muster trägt — der Normalfall "gilt jede Woche" soll die Startseite keine
// zusätzliche Abfrage kosten.
func (s *service) filterByWeekPattern(
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
		if scheduleService.ShouldMaterializeWeekPattern(notice.WeekPattern, date, period) {
			kept = append(kept, notice)
		}
	}
	return kept, nil
}

// periodFor sucht den aktiven Kalenderzeitraum, der den Tag enthält und einen
// Wochenzyklus führt. Ohne Treffer nil — ShouldMaterializeWeekPattern lässt
// den Hinweis dann durch, statt ihn stumm verschwinden zu lassen.
func (s *service) periodFor(ctx context.Context, date timezone.Date) (*scheduleModels.CalendarPeriod, error) {
	periods, err := s.periods.FindActiveByTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("staffnotice: load calendar periods: %w", err)
	}
	for _, period := range periods {
		if period.WeekCycleLength <= 1 || period.WeekCycleAnchor == nil {
			continue
		}
		if date.Before(period.StartDate) || date.After(period.EndDate) {
			continue
		}
		return period, nil
	}
	return nil, nil
}

// decorate hängt an jede Zeile die eigene Kenntnisnahme und die Gesamtzahl der
// Kenntnisnahmen — zwei gebündelte Abfragen für die ganze Liste, kein N+1.
func (s *service) decorate(
	ctx context.Context,
	accountID int64,
	notices []*usersModels.StaffNotice,
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
	counts, err := s.repo.AcknowledgedCounts(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("staffnotice: acknowledgement counts: %w", err)
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

func (s *service) Create(ctx context.Context, createdBy int64, in Input) (*usersModels.StaffNotice, error) {
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

func (s *service) Update(ctx context.Context, id int64, in Input) (*usersModels.StaffNotice, error) {
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

func (s *service) Delete(ctx context.Context, id int64) error {
	if _, err := s.Get(ctx, id); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("staffnotice: delete: %w", err)
	}
	s.logger.Info("staff_notice_deleted", slog.Int64("notice_id", id))
	return nil
}

func (s *service) Acknowledge(ctx context.Context, id, accountID int64) error {
	notice, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if !notice.RequiresAcknowledgement {
		// Ein Hinweis ohne angeforderte Kenntnisnahme hat keine zu speichern.
		// Das ist kein Fehler der Person, sondern eine veraltete Ansicht.
		return fmt.Errorf("%w: notice does not ask for acknowledgement", ErrInvalid)
	}
	today := timezone.TodayDate()
	if !notice.AppliesOn(today) {
		return fmt.Errorf("%w: notice does not apply today", ErrInvalid)
	}
	matching, err := s.filterByWeekPattern(ctx, []*usersModels.StaffNotice{notice}, today)
	if err != nil {
		return err
	}
	if len(matching) == 0 {
		return fmt.Errorf("%w: notice does not apply today", ErrInvalid)
	}
	if err := s.repo.Acknowledge(ctx, id, accountID); err != nil {
		return fmt.Errorf("staffnotice: acknowledge: %w", err)
	}
	return nil
}

// apply überträgt die Eingabe auf die Zeile und prüft sie. Der Zuschnitt der
// Wochentage passiert hier und nicht im Modell: doppelte Einträge sind eine
// Eingabefrage, keine Eigenschaft des Hinweises.
func (s *service) apply(notice *usersModels.StaffNotice, in Input) (*usersModels.StaffNotice, error) {
	notice.Title = strings.TrimSpace(in.Title)
	notice.Body = strings.TrimSpace(in.Body)
	notice.Priority = in.Priority
	if notice.Priority == "" {
		notice.Priority = usersModels.StaffNoticePriorityInfo
	}
	notice.ValidFrom = in.ValidFrom
	notice.ValidUntil = in.ValidUntil
	notice.Weekdays = normalizeWeekdays(in.Weekdays)
	notice.WeekPattern = in.WeekPattern
	notice.RequiresAcknowledgement = in.RequiresAcknowledgement
	notice.Active = in.Active

	if notice.ValidFrom.IsZero() {
		return nil, fmt.Errorf("%w: valid from is required", ErrInvalid)
	}
	if err := notice.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalid, err.Error())
	}
	return notice, nil
}

// normalizeWeekdays sortiert aufsteigend und entfernt Doppelte. Unzulässige
// Werte bleiben absichtlich stehen, damit Validate sie ablehnt, statt sie
// stillschweigend zu schlucken. Alle sieben Tage bedeuten dasselbe wie "keine
// Angabe", werden aber nicht zusammengefasst: die Leitung soll ihre Auswahl
// wiederfinden.
func normalizeWeekdays(in []int16) []int16 {
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
