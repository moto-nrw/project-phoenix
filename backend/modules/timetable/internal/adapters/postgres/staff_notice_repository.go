package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	schedule "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// staffNoticeRepository ist der mandantengebundene Datenzugriff für
// Tagesinformationen (#2180). Timetable besitzt users.staff_notices und
// users.staff_notice_acks; der Adapter bildet die Zeilen auf das
// beibehaltene schedule.StaffNotice ab (#3220) und behält die Fehlerverträge des
// früheren generischen Repositorys bei.
type staffNoticeRepository struct {
	store *Store
}

// NewStaffNoticeRepository serves the retained staff notice repository over
// the owner's store.
func NewStaffNoticeRepository(store *Store) ports.StaffNoticeRecords {
	return staffNoticeRepository{store: store}
}

// Create prüft und legt den Hinweis an; Id, Zeitstempel und Mandant kommen
// aus der Datenbank bzw. dem Kontext zurück.
func (r staffNoticeRepository) Create(ctx context.Context, notice *schedule.StaffNotice) error {
	if notice == nil {
		return fmt.Errorf("StaffNotice cannot be nil or zero value")
	}
	if err := notice.Validate(); err != nil {
		return err
	}
	row := staffNoticeRow(notice)
	if err := r.store.CreateStaffNotice(ctx, row); err != nil {
		return &domain.StoreError{Op: "create", Err: err}
	}
	*notice = *staffNoticeModel(row)
	return nil
}

// Update prüft und schreibt den Hinweis; genau eine Zeile muss sich ändern.
func (r staffNoticeRepository) Update(ctx context.Context, notice *schedule.StaffNotice) error {
	if notice == nil {
		return fmt.Errorf("StaffNotice cannot be nil or zero value")
	}
	if err := notice.Validate(); err != nil {
		return err
	}
	row := staffNoticeRow(notice)
	affected, err := r.store.UpdateStaffNotice(ctx, row)
	if err != nil {
		return &domain.StoreError{Op: "update", Err: err}
	}
	*notice = *staffNoticeModel(row)
	if affected != 1 {
		return &domain.StoreError{
			Op:  "update StaffNotice",
			Err: fmt.Errorf("expected %d rows affected, got %d", 1, affected),
		}
	}
	return nil
}

// FindByID gibt den Hinweis zurück oder nil, wenn es ihn (in diesem Mandanten)
// nicht gibt. "Nicht da" ist hier kein Fehler, sondern die Antwort — der
// Service macht daraus ErrNotFound.
func (r staffNoticeRepository) FindByID(ctx context.Context, id int64) (*schedule.StaffNotice, error) {
	row, found, err := r.store.FindStaffNotice(ctx, id)
	if err != nil {
		return nil, &domain.StoreError{Op: "find by id", Err: err}
	}
	if !found {
		return nil, nil
	}
	return staffNoticeModel(row), nil
}

// Delete entfernt den Hinweis; die Kenntnisnahmen fallen per CASCADE mit.
func (r staffNoticeRepository) Delete(ctx context.Context, id int64) error {
	if err := r.store.DeleteStaffNotice(ctx, id); err != nil {
		return &domain.StoreError{Op: "delete", Err: err}
	}
	return nil
}

// List gibt die Hinweise des Mandanten zurück, neueste zuerst.
func (r staffNoticeRepository) List(ctx context.Context, includeInactive bool) ([]*schedule.StaffNotice, error) {
	rows, err := r.store.ListStaffNotices(ctx, includeInactive)
	if err != nil {
		return nil, &domain.StoreError{Op: "list staff notices", Err: TranslateNotFound(err)}
	}
	return staffNoticeModels(rows), nil
}

// ListValidOn grenzt auf die aktiven Hinweise ein, deren Zeitraum den Tag
// enthält und deren Zielgruppe die Leserart einschließt (#2208). Wochentag und
// Wochenmuster prüft der Service: der Wochentag ist eine Array-Abfrage, die
// kein Index bedient, und das Wochenmuster braucht den Kalenderzeitraum. Die
// Datenbank soll nur die Menge klein machen.
//
// Eine unbekannte Leserart liefert nichts: ein Portal, das sich nicht
// ausweist, bekommt keinen breiteren Verteiler geschenkt.
func (r staffNoticeRepository) ListValidOn(ctx context.Context, date calendar.Date, reader string) ([]*schedule.StaffNotice, error) {
	if !schedule.ValidStaffNoticeReader(reader) {
		return []*schedule.StaffNotice{}, nil
	}
	// Wichtiges zuerst. Nicht nach der Spalte sortieren: alphabetisch käme
	// 'info' vor 'important', der wichtige Hinweis stünde also unten.
	rows, err := r.store.ListStaffNoticesValidOn(ctx, calendar.Date(date.String()), reader, schedule.StaffNoticeAudienceAll, schedule.StaffNoticePriorityImportant)
	if err != nil {
		return nil, &domain.StoreError{Op: "list staff notices valid on date", Err: TranslateNotFound(err)}
	}
	return staffNoticeModels(rows), nil
}

// Acknowledge stempelt die Kenntnisnahme. Ein zweiter Aufruf ändert nichts —
// der erste Zeitpunkt ist der ehrliche.
func (r staffNoticeRepository) Acknowledge(ctx context.Context, noticeID, accountID int64) error {
	ack := &StaffNoticeAck{
		NoticeID:       noticeID,
		AccountID:      accountID,
		AcknowledgedAt: time.Now(),
	}
	if err := r.store.AcknowledgeStaffNotice(ctx, ack); err != nil {
		return &domain.StoreError{Op: "acknowledge staff notice", Err: TranslateNotFound(err)}
	}
	return nil
}

// AcknowledgedAtFor gibt je Hinweis-Id den Zeitpunkt der eigenen Kenntnisnahme
// zurück (eine Abfrage für die ganze Liste).
func (r staffNoticeRepository) AcknowledgedAtFor(ctx context.Context, accountID int64, noticeIDs []int64) (map[int64]time.Time, error) {
	result := make(map[int64]time.Time, len(noticeIDs))
	if len(noticeIDs) == 0 {
		return result, nil
	}
	rows, err := r.store.ListOwnStaffNoticeAcks(ctx, accountID, noticeIDs)
	if err != nil {
		return nil, &domain.StoreError{Op: "load own staff notice acknowledgements", Err: TranslateNotFound(err)}
	}
	for _, row := range rows {
		result[row.NoticeID] = row.AcknowledgedAt
	}
	return result, nil
}

// AcknowledgedCounts gibt je Hinweis-Id die Zahl der Kenntnisnahmen zurück —
// die Antwort auf "ist der Hinweis angekommen".
//
// Die Verfasserin zählt nicht mit. Ihre Kenntnisnahme wird beim Anlegen
// gestempelt, damit der eigene Hinweis sie nicht nach einer Bestätigung fragt;
// als Leserin des Hinweises ist sie damit aber nicht gemeint. Ohne diesen
// Ausschluss stünde bei einem frisch geschriebenen Hinweis „1 Person hat
// bestätigt", und die Leitung liest darin ein Teammitglied.
func (r staffNoticeRepository) AcknowledgedCounts(ctx context.Context, noticeIDs []int64) (map[int64]int, error) {
	result := make(map[int64]int, len(noticeIDs))
	if len(noticeIDs) == 0 {
		return result, nil
	}
	rows, err := r.store.CountStaffNoticeAcks(ctx, noticeIDs)
	if err != nil {
		return nil, &domain.StoreError{Op: "count staff notice acknowledgements", Err: TranslateNotFound(err)}
	}
	for _, row := range rows {
		result[row.NoticeID] = row.Count
	}
	return result, nil
}

// Acknowledgements gibt alle Kenntnisnahmen eines Hinweises zurück, neueste
// zuerst — die Bestätigungsliste der Leitung (#2208). Nur Konto und Zeitpunkt:
// die Namen gehören dem Personenverzeichnis, der Service holt sie dort.
//
// Die Verfasserin fehlt in der Liste aus demselben Grund wie im Zähler
// (AcknowledgedCounts): ihre Kenntnisnahme ist beim Anlegen gestempelt, damit
// der eigene Hinweis sie nicht fragt, aber gelesen hat sie ihn nicht.
func (r staffNoticeRepository) Acknowledgements(ctx context.Context, noticeID int64) ([]*schedule.StaffNoticeAck, error) {
	rows, err := r.store.ListStaffNoticeAcks(ctx, noticeID)
	if err != nil {
		return nil, &domain.StoreError{Op: "list staff notice acknowledgements", Err: TranslateNotFound(err)}
	}
	acks := make([]*schedule.StaffNoticeAck, 0, len(rows))
	for _, row := range rows {
		ack := &schedule.StaffNoticeAck{NoticeID: row.NoticeID, AccountID: row.AccountID, AcknowledgedAt: row.AcknowledgedAt}
		ack.TenantID = row.TenantID
		acks = append(acks, ack)
	}
	return acks, nil
}

func staffNoticeRow(notice *schedule.StaffNotice) *StaffNotice {
	row := &StaffNotice{
		Title:                   notice.Title,
		Body:                    notice.Body,
		Priority:                notice.Priority,
		Audience:                notice.Audience,
		ValidFrom:               calendar.Date(notice.ValidFrom.String()),
		Weekdays:                notice.Weekdays,
		WeekPattern:             notice.WeekPattern,
		RequiresAcknowledgement: notice.RequiresAcknowledgement,
		Active:                  notice.Active,
		CreatedBy:               notice.CreatedBy,
	}
	row.ID = notice.ID
	row.CreatedAt = notice.CreatedAt
	row.UpdatedAt = notice.UpdatedAt
	row.TenantID = notice.TenantID
	if notice.ValidUntil != nil {
		until := calendar.Date(notice.ValidUntil.String())
		row.ValidUntil = &until
	}
	return row
}

func staffNoticeModel(row *StaffNotice) *schedule.StaffNotice {
	notice := &schedule.StaffNotice{
		Title:                   row.Title,
		Body:                    row.Body,
		Priority:                row.Priority,
		Audience:                row.Audience,
		ValidFrom:               calendar.Date(row.ValidFrom),
		Weekdays:                row.Weekdays,
		WeekPattern:             row.WeekPattern,
		RequiresAcknowledgement: row.RequiresAcknowledgement,
		Active:                  row.Active,
		CreatedBy:               row.CreatedBy,
	}
	notice.ID = row.ID
	notice.CreatedAt = row.CreatedAt
	notice.UpdatedAt = row.UpdatedAt
	notice.TenantID = row.TenantID
	if row.ValidUntil != nil {
		until := calendar.Date(*row.ValidUntil)
		notice.ValidUntil = &until
	}
	return notice
}

func staffNoticeModels(rows []*StaffNotice) []*schedule.StaffNotice {
	if rows == nil {
		return nil
	}
	notices := make([]*schedule.StaffNotice, 0, len(rows))
	for _, row := range rows {
		notices = append(notices, staffNoticeModel(row))
	}
	return notices
}
