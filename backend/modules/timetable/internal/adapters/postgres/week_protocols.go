package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

// The weekly protocols of #4769: schedule.group_week_protocols and its days.
const (
	weekProtocolTable    = `schedule.group_week_protocols AS "week_protocol"`
	weekProtocolDayTable = `schedule.group_week_protocol_days AS "week_protocol_day"`
)

type weekProtocolRow struct {
	bun.BaseModel       `bun:"table:schedule.group_week_protocols,alias:week_protocol"`
	ID                  int64         `bun:"id,pk,autoincrement"`
	TenantID            int64         `bun:"tenant_id,notnull"`
	Scope               string        `bun:"scope,notnull"`
	GroupID             *int64        `bun:"group_id"`
	GroupName           string        `bun:"group_name,notnull"`
	WeekStart           calendar.Date `bun:"week_start,notnull,type:date"`
	Status              string        `bun:"status,notnull"`
	WeekNote            string        `bun:"week_note,notnull"`
	DraftedAt           *time.Time    `bun:"drafted_at"`
	ClosedAt            *time.Time    `bun:"closed_at"`
	ClosedByAccountID   *int64        `bun:"closed_by_account_id"`
	ClosedByName        *string       `bun:"closed_by_name"`
	ClosedAutomatically bool          `bun:"closed_automatically,notnull"`
}

type weekProtocolDayRow struct {
	bun.BaseModel `bun:"table:schedule.group_week_protocol_days,alias:week_protocol_day"`
	ID            int64           `bun:"id,pk,autoincrement"`
	TenantID      int64           `bun:"tenant_id,notnull"`
	ProtocolID    int64           `bun:"protocol_id,notnull"`
	Day           calendar.Date   `bun:"day,notnull,type:date"`
	CapturedAt    *time.Time      `bun:"captured_at"`
	Facts         json.RawMessage `bun:"facts,type:jsonb"`
	Note          string          `bun:"note,notnull"`
}

func (s *Store) EnsureWeekProtocol(ctx context.Context, target timetable.WeekProtocolTarget, weekStart calendar.Date) (int64, string, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, "", err
	}
	var row struct {
		ID     int64  `bun:"id"`
		Status string `bun:"status"`
	}
	// DO UPDATE with an unchanged column returns the existing row in the
	// same statement; the kept group name is the one of the first evening.
	if target.GroupID == nil {
		err = db.NewRaw(`
			INSERT INTO schedule.group_week_protocols (tenant_id, scope, group_id, group_name, week_start)
			VALUES (?, 'school', NULL, ?, ?)
			ON CONFLICT (tenant_id, week_start) WHERE scope = 'school' DO UPDATE SET scope = EXCLUDED.scope
			RETURNING id, status`, tenantID, target.GroupName, weekStart).Scan(ctx, &row)
	} else {
		err = db.NewRaw(`
			INSERT INTO schedule.group_week_protocols (tenant_id, scope, group_id, group_name, week_start)
			VALUES (?, 'group', ?, ?, ?)
			ON CONFLICT (tenant_id, group_id, week_start) WHERE scope = 'group' DO UPDATE SET scope = EXCLUDED.scope
			RETURNING id, status`, tenantID, *target.GroupID, target.GroupName, weekStart).Scan(ctx, &row)
	}
	if err != nil {
		return 0, "", fmt.Errorf("ensure week protocol: %w", err)
	}
	return row.ID, row.Status, nil
}

func (s *Store) CaptureWeekProtocolDay(ctx context.Context, protocolID int64, day calendar.Date, facts timetable.WeekProtocolDayFacts, at time.Time) (bool, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, err
	}
	payload, err := json.Marshal(facts)
	if err != nil {
		return false, fmt.Errorf("encode week protocol day: %w", err)
	}
	result, err := db.NewRaw(`
		INSERT INTO schedule.group_week_protocol_days (tenant_id, protocol_id, day, captured_at, facts)
		VALUES (?, ?, ?, ?, ?::jsonb)
		ON CONFLICT (protocol_id, day) DO UPDATE
			SET captured_at = EXCLUDED.captured_at, facts = EXCLUDED.facts
			WHERE schedule.group_week_protocol_days.captured_at IS NULL`,
		tenantID, protocolID, day, at, string(payload)).Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("capture week protocol day: %w", err)
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

// AdvanceWeeks closes the open protocols of weeks before closeBefore and,
// when draftUpTo is set, turns the collecting protocols up to that week into
// drafts. Closing runs first, so a week closed here is never drafted.
func (s *Store) AdvanceWeekProtocols(ctx context.Context, closeBefore calendar.Date, draftUpTo *calendar.Date, at time.Time) (int, int, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, 0, err
	}
	result, err := db.NewUpdate().TableExpr(weekProtocolTable).
		Set("status = ?", timetable.WeekProtocolStatusClosed).
		Set(`drafted_at = COALESCE("week_protocol".drafted_at, ?)`, at).
		Set("closed_at = ?", at).
		Set("closed_automatically = TRUE").
		Where(`"week_protocol".tenant_id = ?`, tenantID).
		Where(`"week_protocol".status <> ?`, timetable.WeekProtocolStatusClosed).
		Where(`"week_protocol".week_start < ?`, closeBefore).
		Exec(ctx)
	closed, err := affectedRows(result, err, "close overdue week protocols")
	if err != nil || draftUpTo == nil {
		return 0, closed, err
	}
	result, err = db.NewUpdate().TableExpr(weekProtocolTable).
		Set("status = ?", timetable.WeekProtocolStatusDraft).
		Set("drafted_at = ?", at).
		Where(`"week_protocol".tenant_id = ?`, tenantID).
		Where(`"week_protocol".status = ?`, timetable.WeekProtocolStatusCollecting).
		Where(`"week_protocol".week_start <= ?`, *draftUpTo).
		Exec(ctx)
	drafted, err := affectedRows(result, err, "draft week protocols")
	return drafted, closed, err
}

func (s *Store) ListWeekProtocols(ctx context.Context, filter timetable.WeekProtocolFilter) ([]timetable.WeekProtocol, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []weekProtocolRow
	query := db.NewSelect().Model(&rows).ModelTableExpr(weekProtocolTable).
		Where(`"week_protocol".tenant_id = ?`, tenantID).
		OrderExpr(`"week_protocol".week_start DESC, "week_protocol".group_name ASC, "week_protocol".id ASC`)
	if filter.GroupID != nil {
		query = query.Where(`"week_protocol".group_id = ?`, *filter.GroupID)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list week protocols: %w", err)
	}
	result := make([]timetable.WeekProtocol, 0, len(rows))
	for _, row := range rows {
		result = append(result, weekProtocolModel(row))
	}
	return result, nil
}

func (s *Store) FindWeekProtocol(ctx context.Context, id int64, lock bool) (timetable.WeekProtocol, bool, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return timetable.WeekProtocol{}, false, err
	}
	row := weekProtocolRow{}
	query := db.NewSelect().Model(&row).ModelTableExpr(weekProtocolTable).
		Where(`"week_protocol".id = ?`, id).
		Where(`"week_protocol".tenant_id = ?`, tenantID)
	if lock {
		query = query.For("UPDATE")
	}
	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return timetable.WeekProtocol{}, false, nil
		}
		return timetable.WeekProtocol{}, false, fmt.Errorf("find week protocol: %w", err)
	}
	var days []weekProtocolDayRow
	if err := db.NewSelect().Model(&days).ModelTableExpr(weekProtocolDayTable).
		Where(`"week_protocol_day".protocol_id = ?`, id).
		Where(`"week_protocol_day".tenant_id = ?`, tenantID).
		OrderExpr(`"week_protocol_day".day ASC`).
		Scan(ctx); err != nil {
		return timetable.WeekProtocol{}, false, fmt.Errorf("list week protocol days: %w", err)
	}
	protocol := weekProtocolModel(row)
	protocol.Days = make([]timetable.WeekProtocolDay, 0, len(days))
	for _, day := range days {
		model, err := weekProtocolDayModel(day)
		if err != nil {
			return timetable.WeekProtocol{}, false, err
		}
		protocol.Days = append(protocol.Days, model)
	}
	return protocol, true, nil
}

func (s *Store) SaveWeekProtocolNotes(ctx context.Context, id int64, notes timetable.WeekProtocolNotes) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	if _, err := db.NewUpdate().TableExpr(weekProtocolTable).
		Set("week_note = ?", notes.WeekNote).
		Where(`"week_protocol".id = ?`, id).
		Where(`"week_protocol".tenant_id = ?`, tenantID).
		Exec(ctx); err != nil {
		return fmt.Errorf("save week protocol note: %w", err)
	}
	for day, note := range notes.Days {
		if _, err := db.NewRaw(`
			INSERT INTO schedule.group_week_protocol_days (tenant_id, protocol_id, day, note)
			VALUES (?, ?, ?, ?)
			ON CONFLICT (protocol_id, day) DO UPDATE SET note = EXCLUDED.note`,
			tenantID, id, day, note).Exec(ctx); err != nil {
			return fmt.Errorf("save week protocol day note: %w", err)
		}
	}
	// A day that carries neither a capture nor a note is no day of the
	// protocol.
	if _, err := db.NewDelete().TableExpr(weekProtocolDayTable).
		Where(`"week_protocol_day".protocol_id = ?`, id).
		Where(`"week_protocol_day".tenant_id = ?`, tenantID).
		Where(`"week_protocol_day".captured_at IS NULL`).
		Where(`"week_protocol_day".note = ''`).
		Exec(ctx); err != nil {
		return fmt.Errorf("drop empty week protocol days: %w", err)
	}
	return nil
}

func (s *Store) CloseWeekProtocol(ctx context.Context, id int64, actor timetable.WeekProtocolActor, at time.Time) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	name := actor.Name
	_, err = db.NewUpdate().TableExpr(weekProtocolTable).
		Set("status = ?", timetable.WeekProtocolStatusClosed).
		Set("closed_at = ?", at).
		Set("closed_by_account_id = ?", actor.AccountID).
		Set("closed_by_name = ?", &name).
		Where(`"week_protocol".id = ?`, id).
		Where(`"week_protocol".tenant_id = ?`, tenantID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("close week protocol: %w", err)
	}
	return nil
}

func (s *Store) DeleteWeekProtocolsEndedBefore(ctx context.Context, cutoff calendar.Date) (int, error) {
	if cutoff.IsZero() {
		// A zero day would bind an empty DATE; refuse it rather than let the
		// database decide what an unset cutoff deletes.
		return 0, fmt.Errorf("delete expired week protocols: the cutoff is required")
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, err
	}
	// The week ends on its Sunday, six days after week_start.
	result, err := db.NewDelete().TableExpr(weekProtocolTable).
		Where(`"week_protocol".tenant_id = ?`, tenantID).
		Where(`"week_protocol".week_start < ?`, cutoff.AddDays(-6)).
		Exec(ctx)
	return affectedRows(result, err, "delete expired week protocols")
}

func affectedRows(result sql.Result, err error, operation string) (int, error) {
	if err != nil {
		return 0, fmt.Errorf("%s: %w", operation, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("%s: %w", operation, err)
	}
	return int(affected), nil
}

func weekProtocolModel(row weekProtocolRow) timetable.WeekProtocol {
	protocol := timetable.WeekProtocol{
		ID: row.ID, Scope: row.Scope, GroupID: row.GroupID, GroupName: row.GroupName,
		WeekStart: row.WeekStart, WeekEnd: row.WeekStart.AddDays(6), Status: row.Status, WeekNote: row.WeekNote,
		DraftedAt: row.DraftedAt, ClosedAt: row.ClosedAt, ClosedAutomatically: row.ClosedAutomatically,
	}
	if row.ClosedByName != nil {
		protocol.ClosedByName = *row.ClosedByName
	}
	return protocol
}

func weekProtocolDayModel(row weekProtocolDayRow) (timetable.WeekProtocolDay, error) {
	day := timetable.WeekProtocolDay{Date: row.Day, CapturedAt: row.CapturedAt, Note: row.Note}
	if len(row.Facts) == 0 || string(row.Facts) == "null" {
		return day, nil
	}
	facts := timetable.WeekProtocolDayFacts{}
	if err := json.Unmarshal(row.Facts, &facts); err != nil {
		return timetable.WeekProtocolDay{}, fmt.Errorf("decode week protocol day %s: %w", row.Day, err)
	}
	day.Facts = &facts
	return day, nil
}
