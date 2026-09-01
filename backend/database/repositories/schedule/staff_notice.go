package schedule

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// StaffNoticeRepository ist der mandantengebundene Datenzugriff für
// Tagesinformationen (#2180). Die CRUD-Fläche kommt aus base.Repository; nur
// die Kenntnisnahmen brauchen eigene Abfragen, weil sie in einer zweiten
// Tabelle liegen.
type StaffNoticeRepository struct {
	*base.Repository[*users.StaffNotice]
}

// NewStaffNoticeRepository verdrahtet ein frisches Repository.
func NewStaffNoticeRepository(db *bun.DB) users.StaffNoticeRepository {
	repo := base.NewRepository[*users.StaffNotice](db, "users.staff_notices", "StaffNotice")
	repo.TenantScoped = true
	return &StaffNoticeRepository{Repository: repo}
}

// FindByID gibt den Hinweis zurück oder nil, wenn es ihn (in diesem Mandanten)
// nicht gibt. "Nicht da" ist hier kein Fehler, sondern die Antwort — der
// Service macht daraus ErrNotFound.
func (r *StaffNoticeRepository) FindByID(ctx context.Context, id int64) (*users.StaffNotice, error) {
	return r.FindByIDOrNil(ctx, id)
}

// Delete entfernt den Hinweis; die Kenntnisnahmen fallen per CASCADE mit.
func (r *StaffNoticeRepository) Delete(ctx context.Context, id int64) error {
	return r.Repository.Delete(ctx, id)
}

// List gibt die Hinweise des Mandanten zurück, neueste zuerst.
func (r *StaffNoticeRepository) List(ctx context.Context, includeInactive bool) ([]*users.StaffNotice, error) {
	var rows []*users.StaffNotice
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(`users.staff_notices AS "staff_notice"`).
		OrderExpr(`"staff_notice".created_at DESC`)
	if !includeInactive {
		query = query.Where(`"staff_notice".active`)
	}
	query = base.WithTenantFilter(ctx, query, "staff_notice")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list staff notices", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}

// ListValidOn grenzt auf die aktiven Hinweise ein, deren Zeitraum den Tag
// enthält. Wochentag und Wochenmuster prüft der Service: der Wochentag ist eine
// Array-Abfrage, die kein Index bedient, und das Wochenmuster braucht den
// Kalenderzeitraum. Die Datenbank soll nur die Menge klein machen.
func (r *StaffNoticeRepository) ListValidOn(ctx context.Context, date timezone.Date) ([]*users.StaffNotice, error) {
	var rows []*users.StaffNotice
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(`users.staff_notices AS "staff_notice"`).
		Where(`"staff_notice".active`).
		Where(`"staff_notice".valid_from <= ?`, date).
		Where(`("staff_notice".valid_until IS NULL OR "staff_notice".valid_until >= ?)`, date).
		// Wichtiges zuerst. Nicht nach der Spalte sortieren: alphabetisch käme
		// 'info' vor 'important', der wichtige Hinweis stünde also unten.
		OrderExpr(`CASE WHEN "staff_notice".priority = ? THEN 0 ELSE 1 END, "staff_notice".created_at DESC`,
			users.StaffNoticePriorityImportant)
	query = base.WithTenantFilter(ctx, query, "staff_notice")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list staff notices valid on date", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}

// Acknowledge stempelt die Kenntnisnahme. Ein zweiter Aufruf ändert nichts —
// der erste Zeitpunkt ist der ehrliche.
func (r *StaffNoticeRepository) Acknowledge(ctx context.Context, noticeID, accountID int64) error {
	ack := &users.StaffNoticeAck{
		NoticeID:       noticeID,
		AccountID:      accountID,
		AcknowledgedAt: time.Now(),
	}
	base.EnsureTenantID(ctx, ack)
	if _, err := base.GetDB(ctx, r.DB).NewInsert().
		Model(ack).
		ModelTableExpr("users.staff_notice_acks").
		On("CONFLICT (notice_id, account_id) DO NOTHING").
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "acknowledge staff notice", Err: base.TranslateNotFound(err)}
	}
	return nil
}

// AcknowledgedAtFor gibt je Hinweis-Id den Zeitpunkt der eigenen Kenntnisnahme
// zurück (eine Abfrage für die ganze Liste).
func (r *StaffNoticeRepository) AcknowledgedAtFor(ctx context.Context, accountID int64, noticeIDs []int64) (map[int64]time.Time, error) {
	result := make(map[int64]time.Time, len(noticeIDs))
	if len(noticeIDs) == 0 {
		return result, nil
	}
	var rows []*users.StaffNoticeAck
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(`users.staff_notice_acks AS "sna"`).
		Where(`"sna".account_id = ?`, accountID).
		Where(`"sna".notice_id IN (?)`, bun.List(noticeIDs))
	query = base.WithTenantFilter(ctx, query, "sna")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "load own staff notice acknowledgements", Err: base.TranslateNotFound(err)}
	}
	for _, row := range rows {
		result[row.NoticeID] = row.AcknowledgedAt
	}
	return result, nil
}

// AcknowledgedCounts gibt je Hinweis-Id die Zahl der Kenntnisnahmen zurück —
// die Antwort auf "ist der Hinweis angekommen".
func (r *StaffNoticeRepository) AcknowledgedCounts(ctx context.Context, noticeIDs []int64) (map[int64]int, error) {
	result := make(map[int64]int, len(noticeIDs))
	if len(noticeIDs) == 0 {
		return result, nil
	}
	var rows []struct {
		NoticeID int64 `bun:"notice_id"`
		Count    int   `bun:"count"`
	}
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model((*users.StaffNoticeAck)(nil)).
		ModelTableExpr(`users.staff_notice_acks AS "sna"`).
		ColumnExpr(`"sna".notice_id AS notice_id`).
		ColumnExpr("COUNT(*) AS count").
		Where(`"sna".notice_id IN (?)`, bun.List(noticeIDs)).
		GroupExpr(`"sna".notice_id`)
	query = base.WithTenantFilter(ctx, query, "sna")
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "count staff notice acknowledgements", Err: base.TranslateNotFound(err)}
	}
	for _, row := range rows {
		result[row.NoticeID] = row.Count
	}
	return result, nil
}
