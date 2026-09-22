package compose

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/legacy/timetableplanning"
	"github.com/uptrace/bun"
)

// Tagesinformationen (#2180) — Geschäftslogik der Hinweise fürs Team:
// interner Hinweise der Leitung an das Team, die an bestimmten Tagen gelten.
//
// Wichtig für die Einordnung: hier entsteht KEINE zweite Recurrence-Engine. Ein
// Hinweis wird nie in Tageszeilen materialisiert, er wird beim Lesen gegen das
// Datum geprüft. Das Vokabular (Wochentage, Wochenmuster, Gültigkeitszeitraum)
// und die Auswertung des Wochenmusters kommen aus dem Stundenplan
// (schedule.schedule.ShouldMaterializeWeekPattern), damit "Woche A" hier dasselbe heißt
// wie dort.

// StaffNoticePeriodLookup ist der Ausschnitt des Kalenderzeitraum-Repositories, den die
// Auflösung des Wochenmusters braucht. Bewusst hier deklariert und nicht das
// volle Repository verlangt: der Dienst liest Zeiträume, er verwaltet keine.
type StaffNoticePeriodLookup interface {
	FindActiveByTenantID(ctx context.Context) ([]*scheduleModels.CalendarPeriod, error)
}

// StaffNoticeDependencies ist das Abhängigkeitsbündel. Periods ist optional: ohne
// Kalenderzeitraum lässt sich kein Wochenmuster auflösen, dann gilt ein Hinweis
// in jeder Woche (dieselbe Richtung wie schedule.ShouldMaterializeWeekPattern). Names ist
// optional: ohne Verzeichnis zeigt die Bestätigungsliste den Platzhalter. Names
// gibt je Konto-Id den Anzeigenamen der aktiven Person des Mandanten zurück;
// Konten ohne Person fehlen in der Antwort.
type StaffNoticeDependencies struct {
	DB          *bun.DB
	Periods     StaffNoticePeriodLookup
	Names       func(ctx context.Context, accountIDs []int64) (map[int64]string, error)
	Logger      *slog.Logger
	CurrentDate func() timezone.Date
}

type staffNoticeService struct {
	repo        usersModels.StaffNoticeRepository
	periods     StaffNoticePeriodLookup
	names       func(ctx context.Context, accountIDs []int64) (map[int64]string, error)
	logger      *slog.Logger
	currentDate func() timezone.Date
}

// NewStaffNotices verdrahtet den Dienst auf dem eigenen Repository des Owners.
func NewStaffNotices(deps StaffNoticeDependencies) timetable.StaffNotices {
	return newStaffNotices(NewStaffNoticeRepository(deps.DB), deps)
}

// newStaffNotices ist die Naht für die Logiktests: sie setzen ein anderes
// Repository ein, ohne dass der Dienst eine Datenbank braucht.
func newStaffNotices(repo usersModels.StaffNoticeRepository, deps StaffNoticeDependencies) *staffNoticeService {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	currentDate := deps.CurrentDate
	if currentDate == nil {
		currentDate = timezone.TodayDate
	}
	return &staffNoticeService{repo: repo, periods: deps.Periods, names: deps.Names, logger: logger, currentDate: currentDate}
}

func (s *staffNoticeService) FindStaffNotice(ctx context.Context, id int64) (timetable.StaffNotice, error) {
	notice, err := s.find(ctx, id)
	if err != nil {
		return timetable.StaffNotice{}, err
	}
	return staffNoticeContract(notice), nil
}

// find gibt die Zeile zurück, mit der die Geschäftslogik weiterrechnet;
// FindStaffNotice reicht davon nur die öffentliche Form nach außen.
func (s *staffNoticeService) find(ctx context.Context, id int64) (*usersModels.StaffNotice, error) {
	notice, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("staffnotice: get: %w", err)
	}
	if notice == nil {
		return nil, timetable.ErrStaffNoticeNotFound
	}
	return notice, nil
}

func (s *staffNoticeService) ListStaffNotices(ctx context.Context, accountID int64, includeInactive bool) ([]timetable.StaffNoticeView, error) {
	rows, err := s.repo.List(ctx, includeInactive)
	if err != nil {
		return nil, fmt.Errorf("staffnotice: list: %w", err)
	}
	return s.decorate(ctx, accountID, rows, true)
}

func (s *staffNoticeService) StaffNoticesOn(ctx context.Context, accountID int64, date string, reader string) ([]timetable.StaffNoticeView, error) {
	day, err := timezone.ParseDate(date)
	if err != nil {
		return nil, fmt.Errorf("%w: date must be a date (YYYY-MM-DD)", timetable.ErrStaffNoticeInvalid)
	}
	if !usersModels.ValidStaffNoticeReader(reader) {
		// Kein Portal, keine Hinweise: eine unbekannte Leserart darf nicht
		// den breitesten Verteiler bekommen.
		return []timetable.StaffNoticeView{}, nil
	}
	rows, err := s.repo.ListValidOn(ctx, day, reader)
	if err != nil {
		return nil, fmt.Errorf("staffnotice: today: %w", err)
	}

	matching := make([]*usersModels.StaffNotice, 0, len(rows))
	for _, notice := range rows {
		if !notice.AppliesOn(day) || !notice.AppliesTo(reader) {
			continue
		}
		matching = append(matching, notice)
	}

	matching, err = s.filterByWeekPattern(ctx, matching, day)
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
		if timetableplanning.ShouldMaterializeWeekPattern(notice.WeekPattern, date, period) {
			kept = append(kept, notice)
		}
	}
	return kept, nil
}

// periodFor sucht das aktive Schuljahr, das den Tag enthält und einen
// Wochenzyklus führt. Für Tagesinformationen ist das Schuljahr der eindeutige
// Träger von Woche A/B: Ferien, Halbjahre und eigene Zeiträume dürfen sich
// damit überschneiden, ohne die Wiederholung zu verändern. Ohne Treffer nil —
// schedule.ShouldMaterializeWeekPattern lässt den Hinweis dann durch, statt ihn stumm
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
) ([]timetable.StaffNoticeView, error) {
	views := make([]timetable.StaffNoticeView, 0, len(notices))
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
		view := timetable.StaffNoticeView{
			StaffNotice:       staffNoticeContract(notice),
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

func (s *staffNoticeService) CreateStaffNotice(ctx context.Context, createdBy int64, in timetable.StaffNoticeInput) (timetable.StaffNotice, error) {
	notice, err := s.apply(&usersModels.StaffNotice{CreatedBy: createdBy}, in)
	if err != nil {
		return timetable.StaffNotice{}, err
	}
	if err := s.repo.Create(ctx, notice); err != nil {
		return timetable.StaffNotice{}, fmt.Errorf("staffnotice: create: %w", err)
	}
	// Wer den Hinweis schreibt, kennt ihn. Ohne diese Zeile fragt die eigene
	// Tagesinformation die Leitung nach einer Kenntnisnahme und zählt so lange
	// im Badge mit — eine Aufgabe, die niemand erledigen kann, weil sie keine
	// ist. Die Kenntnisnahme hier zu stempeln hält jeden Leseweg (Startseite,
	// Liste, Badge, Zähler) ohne Sonderfall richtig.
	if notice.RequiresAcknowledgement {
		if err := s.repo.Acknowledge(ctx, notice.ID, createdBy); err != nil {
			return timetable.StaffNotice{}, fmt.Errorf("staffnotice: acknowledge author: %w", err)
		}
	}
	s.logger.Info("staff_notice_created",
		slog.Int64("notice_id", notice.ID),
		slog.Int64("created_by", createdBy),
		slog.String("priority", notice.Priority),
		slog.String("audience", notice.Audience),
	)
	return staffNoticeContract(notice), nil
}

func (s *staffNoticeService) UpdateStaffNotice(ctx context.Context, id int64, in timetable.StaffNoticeInput) (timetable.StaffNotice, error) {
	existing, err := s.find(ctx, id)
	if err != nil {
		return timetable.StaffNotice{}, err
	}
	notice, err := s.apply(existing, in)
	if err != nil {
		return timetable.StaffNotice{}, err
	}
	if err := s.repo.Update(ctx, notice); err != nil {
		return timetable.StaffNotice{}, fmt.Errorf("staffnotice: update: %w", err)
	}
	// Wird die Kenntnisnahme erst nachträglich verlangt, gilt dasselbe wie beim
	// Anlegen: die Verfasserin muss ihren eigenen Hinweis nicht bestätigen.
	if notice.RequiresAcknowledgement {
		if err := s.repo.Acknowledge(ctx, notice.ID, notice.CreatedBy); err != nil {
			return timetable.StaffNotice{}, fmt.Errorf("staffnotice: acknowledge author: %w", err)
		}
	}
	s.logger.Info("staff_notice_updated", slog.Int64("notice_id", notice.ID))
	return staffNoticeContract(notice), nil
}

func (s *staffNoticeService) DeleteStaffNotice(ctx context.Context, id int64) error {
	if _, err := s.find(ctx, id); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("staffnotice: delete: %w", err)
	}
	s.logger.Info("staff_notice_deleted", slog.Int64("notice_id", id))
	return nil
}

func (s *staffNoticeService) AcknowledgeStaffNotice(ctx context.Context, id, accountID int64, reader string) error {
	notice, err := s.find(ctx, id)
	if err != nil {
		return err
	}
	if !notice.AppliesTo(reader) {
		// Ein Hinweis, der dieses Portal nicht erreicht, existiert für die
		// Person dort nicht — dieselbe Antwort wie für einen fremden Hinweis,
		// damit die Zielgruppe nicht über die Bestätigung erratbar wird.
		return timetable.ErrStaffNoticeNotFound
	}
	if !notice.RequiresAcknowledgement {
		// Ein Hinweis ohne angeforderte Kenntnisnahme hat keine zu speichern.
		// Das ist kein Fehler der Person, sondern eine veraltete Ansicht.
		return fmt.Errorf("%w: notice does not ask for acknowledgement", timetable.ErrStaffNoticeInvalid)
	}
	today := s.currentDate()
	if !notice.AppliesOn(today) {
		return fmt.Errorf("%w: notice does not apply today", timetable.ErrStaffNoticeInvalid)
	}
	matching, err := s.filterByWeekPattern(ctx, []*usersModels.StaffNotice{notice}, today)
	if err != nil {
		return err
	}
	if len(matching) == 0 {
		return fmt.Errorf("%w: notice does not apply today", timetable.ErrStaffNoticeInvalid)
	}
	if err := s.repo.Acknowledge(ctx, id, accountID); err != nil {
		return fmt.Errorf("staffnotice: acknowledge: %w", err)
	}
	return nil
}

// StaffNoticeAcknowledgers löst die Kenntnisnahmen eines Hinweises zu Namen
// auf. Eine
// Abfrage für die Liste, eine für die Namen — kein N+1. Wer im Verzeichnis
// nicht mehr steht, bleibt als Platzhalter in der Liste: die Bestätigung war
// echt, auch wenn das Konto inzwischen weg ist.
func (s *staffNoticeService) StaffNoticeAcknowledgers(ctx context.Context, id int64) ([]timetable.StaffNoticeAcknowledger, error) {
	if _, err := s.find(ctx, id); err != nil {
		return nil, err
	}
	acks, err := s.repo.Acknowledgements(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("staffnotice: acknowledgements: %w", err)
	}
	out := make([]timetable.StaffNoticeAcknowledger, 0, len(acks))
	if len(acks) == 0 {
		return out, nil
	}

	names := map[int64]string{}
	if s.names != nil {
		ids := make([]int64, 0, len(acks))
		for _, ack := range acks {
			ids = append(ids, ack.AccountID)
		}
		names, err = s.names(ctx, ids)
		if err != nil {
			return nil, fmt.Errorf("staffnotice: acknowledger names: %w", err)
		}
	}

	for _, ack := range acks {
		name := strings.TrimSpace(names[ack.AccountID])
		if name == "" {
			name = timetable.StaffNoticeUnknownAcknowledgerName
		}
		out = append(out, timetable.StaffNoticeAcknowledger{
			AccountID:      ack.AccountID,
			Name:           name,
			AcknowledgedAt: ack.AcknowledgedAt,
		})
	}
	return out, nil
}

// apply überträgt die Eingabe auf die Zeile und prüft sie. Der Zuschnitt der
// Wochentage passiert hier und nicht im Modell: doppelte Einträge sind eine
// Eingabefrage, keine Eigenschaft des Hinweises.
func (s *staffNoticeService) apply(notice *usersModels.StaffNotice, in timetable.StaffNoticeInput) (*usersModels.StaffNotice, error) {
	validFrom, err := parseNoticeDate(in.ValidFrom)
	if err != nil {
		return nil, fmt.Errorf("%w: valid from must be a date (YYYY-MM-DD)", timetable.ErrStaffNoticeInvalid)
	}
	validUntil, err := parseNoticeDate(in.ValidUntil)
	if err != nil {
		return nil, fmt.Errorf("%w: valid until must be a date (YYYY-MM-DD)", timetable.ErrStaffNoticeInvalid)
	}

	notice.Title = strings.TrimSpace(in.Title)
	notice.Body = strings.TrimSpace(in.Body)
	notice.Priority = in.Priority
	if notice.Priority == "" {
		notice.Priority = usersModels.StaffNoticePriorityInfo
	}
	notice.Audience = in.Audience
	if notice.Audience == "" {
		notice.Audience = usersModels.StaffNoticeAudienceAll
	}
	notice.ValidFrom = validFrom
	// Ein leeres ValidUntil heißt unbefristet, nicht "gilt bis zum Nulltag".
	notice.ValidUntil = nil
	if !validUntil.IsZero() {
		until := validUntil
		notice.ValidUntil = &until
	}
	notice.Weekdays = normalizeNoticeWeekdays(in.Weekdays)
	notice.WeekPattern = in.WeekPattern
	notice.RequiresAcknowledgement = in.RequiresAcknowledgement
	notice.Active = in.Active

	if notice.ValidFrom.IsZero() {
		return nil, fmt.Errorf("%w: valid from is required", timetable.ErrStaffNoticeInvalid)
	}
	if err := notice.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", timetable.ErrStaffNoticeInvalid, err.Error())
	}
	return notice, nil
}

// parseNoticeDate liest einen Kalendertag der öffentlichen Schreibform. Leer
// ist kein Fehler, sondern "nicht gesetzt" — welches der beiden Daten das sein
// darf, entscheidet apply.
func parseNoticeDate(value string) (timezone.Date, error) {
	if value == "" {
		return "", nil
	}
	return timezone.ParseDate(value)
}

// staffNoticeContract bildet die beibehaltene Zeile auf die öffentliche Form
// ab: Kalendertage als YYYY-MM-DD, ein leeres ValidUntil heißt unbefristet.
func staffNoticeContract(notice *usersModels.StaffNotice) timetable.StaffNotice {
	if notice == nil {
		return timetable.StaffNotice{}
	}
	out := timetable.StaffNotice{
		ID:                      notice.ID,
		TenantID:                notice.TenantID,
		Title:                   notice.Title,
		Body:                    notice.Body,
		Priority:                notice.Priority,
		Audience:                notice.Audience,
		ValidFrom:               notice.ValidFrom.String(),
		Weekdays:                notice.Weekdays,
		WeekPattern:             notice.WeekPattern,
		RequiresAcknowledgement: notice.RequiresAcknowledgement,
		Active:                  notice.Active,
		CreatedBy:               notice.CreatedBy,
		CreatedAt:               notice.CreatedAt,
		UpdatedAt:               notice.UpdatedAt,
	}
	if notice.ValidUntil != nil {
		out.ValidUntil = notice.ValidUntil.String()
	}
	return out
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
