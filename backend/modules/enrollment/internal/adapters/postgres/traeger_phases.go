package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// Träger phases (#4704, ADR 0195). Like the holiday periods their tables
// carry organization_id and a Träger policy: every statement runs in the
// Träger transaction of the Träger it names (traegerDB) and names it once
// more in its own filter.

// traegerCatalogSchoolConstraint is the name the school trigger of the
// selection tables raises for a school outside the Träger.
const traegerCatalogSchoolConstraint = "chk_traeger_catalog_school"

// TraegerPhaseStore persists Träger phases in the Träger transaction.
type TraegerPhaseStore struct{}

// NewTraegerPhaseStore builds the store.
func NewTraegerPhaseStore() *TraegerPhaseStore { return &TraegerPhaseStore{} }

type traegerPhaseRow struct {
	ID              int64           `bun:"id"`
	OrganizationID  int64           `bun:"organization_id"`
	Draft           json.RawMessage `bun:"draft"`
	AllSchools      bool            `bun:"all_schools"`
	ReleasedVersion *int            `bun:"released_version"`
	Released        json.RawMessage `bun:"released"`
	WithdrawnAt     *time.Time      `bun:"withdrawn_at"`
	UpdatedAt       time.Time       `bun:"updated_at"`
}

type traegerCatalogSchoolRow struct {
	ParentID       int64      `bun:"parent_id"`
	SchoolID       int64      `bun:"school_id"`
	Selected       bool       `bun:"selected"`
	AppliedVersion *int       `bun:"applied_version"`
	Outcome        *string    `bun:"outcome"`
	ErrorCode      *string    `bun:"error_code"`
	RecordedAt     *time.Time `bun:"recorded_at"`
}

const traegerPhaseSelect = `SELECT phase.id, phase.organization_id, phase.draft, phase.all_schools, phase.released_version,
	version.fields AS released, phase.withdrawn_at, phase.updated_at
	FROM enrollment.traeger_phases AS phase
	LEFT JOIN enrollment.traeger_phase_versions AS version
		ON version.organization_id = phase.organization_id AND version.traeger_phase_id = phase.id
		AND version.version = phase.released_version
	WHERE phase.organization_id = ?`

const (
	traegerPhaseByIDSelect     = traegerPhaseSelect + ` AND phase.id = ?`
	traegerPhaseByIDLockSelect = traegerPhaseByIDSelect + ` FOR UPDATE OF phase`
)

// ListTraegerPhases returns every phase of the Träger with its schools in
// two statements.
func (s *TraegerPhaseStore) ListTraegerPhases(ctx context.Context, organizationID int64) ([]enrollment.TraegerPhase, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []traegerPhaseRow
	if err := db.NewRaw(traegerPhaseSelect+` ORDER BY phase.id DESC`, organizationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: list träger phases: %w", err)
	}
	return s.withSchools(ctx, db, organizationID, rows)
}

// TraegerPhase returns one phase of the Träger.
func (s *TraegerPhaseStore) TraegerPhase(ctx context.Context, organizationID, id int64) (enrollment.TraegerPhase, error) {
	return s.readTraegerPhase(ctx, organizationID, id, false)
}

// LockTraegerPhase returns one phase under its row lock.
func (s *TraegerPhaseStore) LockTraegerPhase(ctx context.Context, organizationID, id int64) (enrollment.TraegerPhase, error) {
	return s.readTraegerPhase(ctx, organizationID, id, true)
}

func (s *TraegerPhaseStore) readTraegerPhase(ctx context.Context, organizationID, id int64, lock bool) (enrollment.TraegerPhase, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return enrollment.TraegerPhase{}, err
	}
	var rows []traegerPhaseRow
	if lock {
		err = db.NewRaw(traegerPhaseByIDLockSelect, organizationID, id).Scan(ctx, &rows)
	} else {
		err = db.NewRaw(traegerPhaseByIDSelect, organizationID, id).Scan(ctx, &rows)
	}
	if err != nil {
		return enrollment.TraegerPhase{}, fmt.Errorf("enrollment postgres: read träger phase: %w", err)
	}
	if len(rows) == 0 {
		return enrollment.TraegerPhase{}, enrollment.ErrTraegerPhaseNotFound
	}
	phases, err := s.withSchools(ctx, db, organizationID, rows)
	if err != nil {
		return enrollment.TraegerPhase{}, err
	}
	return phases[0], nil
}

func (s *TraegerPhaseStore) withSchools(ctx context.Context, db bun.IDB, organizationID int64, rows []traegerPhaseRow) ([]enrollment.TraegerPhase, error) {
	phases := make([]enrollment.TraegerPhase, 0, len(rows))
	if len(rows) == 0 {
		return phases, nil
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	var schools []traegerCatalogSchoolRow
	if err := db.NewRaw(`SELECT traeger_phase_id AS parent_id, school_id, selected, applied_version, outcome, error_code, recorded_at
		FROM enrollment.traeger_phase_schools WHERE organization_id = ? AND traeger_phase_id IN (?) ORDER BY school_id`,
		organizationID, bun.List(ids)).Scan(ctx, &schools); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read träger phase schools: %w", err)
	}
	byPhase := traegerSchoolsByParent(schools)
	for _, row := range rows {
		phase, err := row.value()
		if err != nil {
			return nil, err
		}
		phase.Schools = byPhase[row.ID]
		if phase.Schools == nil {
			phase.Schools = []enrollment.TraegerCatalogSchool{}
		}
		phases = append(phases, phase)
	}
	return phases, nil
}

func (row traegerPhaseRow) value() (enrollment.TraegerPhase, error) {
	phase := enrollment.TraegerPhase{
		ID: row.ID, OrganizationID: row.OrganizationID, AllSchools: row.AllSchools,
		ReleasedVersion: row.ReleasedVersion, WithdrawnAt: row.WithdrawnAt, UpdatedAt: row.UpdatedAt,
	}
	if err := json.Unmarshal(row.Draft, &phase.Draft); err != nil {
		return phase, fmt.Errorf("enrollment postgres: decode träger phase draft: %w", err)
	}
	if len(row.Released) > 0 {
		var released enrollment.TraegerPhaseFields
		if err := json.Unmarshal(row.Released, &released); err != nil {
			return phase, fmt.Errorf("enrollment postgres: decode träger phase version: %w", err)
		}
		phase.Released = &released
	}
	return phase, nil
}

func traegerSchoolsByParent(rows []traegerCatalogSchoolRow) map[int64][]enrollment.TraegerCatalogSchool {
	byParent := make(map[int64][]enrollment.TraegerCatalogSchool)
	for _, row := range rows {
		school := enrollment.TraegerCatalogSchool{
			SchoolID: row.SchoolID, Selected: row.Selected, AppliedVersion: row.AppliedVersion, RecordedAt: row.RecordedAt,
		}
		if row.Outcome != nil {
			school.Outcome = *row.Outcome
		}
		if row.ErrorCode != nil {
			school.ErrorCode = *row.ErrorCode
		}
		byParent[row.ParentID] = append(byParent[row.ParentID], school)
	}
	return byParent
}

// InsertTraegerPhase creates the phase with its draft and returns its ID.
func (s *TraegerPhaseStore) InsertTraegerPhase(ctx context.Context, organizationID int64, draft enrollment.TraegerPhaseFields, allSchools bool) (int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	encoded, err := json.Marshal(draft)
	if err != nil {
		return 0, fmt.Errorf("enrollment postgres: encode träger phase draft: %w", err)
	}
	var id int64
	if err := db.NewRaw(`INSERT INTO enrollment.traeger_phases (organization_id, draft, all_schools) VALUES (?, ?, ?) RETURNING id`,
		organizationID, string(encoded), allSchools).Scan(ctx, &id); err != nil {
		return 0, fmt.Errorf("enrollment postgres: insert träger phase: %w", err)
	}
	return id, nil
}

// UpdateTraegerPhaseDraft replaces the draft of a phase that is not
// withdrawn; false reports that no such phase exists.
func (s *TraegerPhaseStore) UpdateTraegerPhaseDraft(ctx context.Context, organizationID, id int64, draft enrollment.TraegerPhaseFields, allSchools bool) (bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return false, err
	}
	encoded, err := json.Marshal(draft)
	if err != nil {
		return false, fmt.Errorf("enrollment postgres: encode träger phase draft: %w", err)
	}
	result, err := db.NewRaw(`UPDATE enrollment.traeger_phases SET draft = ?, all_schools = ?
		WHERE organization_id = ? AND id = ? AND withdrawn_at IS NULL`, string(encoded), allSchools, organizationID, id).Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("enrollment postgres: update träger phase: %w", err)
	}
	return affectedOne(result)
}

// ReplaceTraegerPhaseSchools selects exactly schoolIDs. A school that left
// the selection keeps its row while it reports a copy, so the next rollout
// withdraws the copy there; a school that never got one goes.
func (s *TraegerPhaseStore) ReplaceTraegerPhaseSchools(ctx context.Context, organizationID, id int64, schoolIDs []int64) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(`DELETE FROM enrollment.traeger_phase_schools
		WHERE organization_id = ? AND traeger_phase_id = ? AND NOT (school_id = ANY(?::bigint[]))
			AND ((applied_version IS NULL AND recorded_at IS NULL) OR outcome = 'withdrawn')`, organizationID, id, pgdialect.Array(schoolIDs)).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: drop träger phase schools: %w", err)
	}
	if _, err := db.NewRaw(`UPDATE enrollment.traeger_phase_schools SET selected = (school_id = ANY(?::bigint[]))
		WHERE organization_id = ? AND traeger_phase_id = ?`, pgdialect.Array(schoolIDs), organizationID, id).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: deselect träger phase schools: %w", err)
	}
	if len(schoolIDs) == 0 {
		return nil
	}
	_, err = db.NewRaw(`INSERT INTO enrollment.traeger_phase_schools (organization_id, traeger_phase_id, school_id)
		SELECT ?, ?, unnest(?::bigint[]) ON CONFLICT (organization_id, traeger_phase_id, school_id) DO UPDATE SET selected = true`,
		organizationID, id, pgdialect.Array(schoolIDs)).Exec(ctx)
	return markTraegerSchoolError(err, "select träger phase schools")
}

// WithdrawTraegerPhase marks the phase withdrawn; false reports that no
// phase exists.
func (s *TraegerPhaseStore) WithdrawTraegerPhase(ctx context.Context, organizationID, id int64) (bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return false, err
	}
	result, err := db.NewRaw(`UPDATE enrollment.traeger_phases SET withdrawn_at = COALESCE(withdrawn_at, now())
		WHERE organization_id = ? AND id = ?`, organizationID, id).Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("enrollment postgres: withdraw träger phase: %w", err)
	}
	return affectedOne(result)
}

// InsertTraegerPhaseVersion writes version as released by accountID and
// makes it the released version of the phase.
func (s *TraegerPhaseStore) InsertTraegerPhaseVersion(ctx context.Context, organizationID, id int64, version int, fields enrollment.TraegerPhaseFields, accountID int64) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("enrollment postgres: encode träger phase version: %w", err)
	}
	if _, err := db.NewRaw(`INSERT INTO enrollment.traeger_phase_versions (organization_id, traeger_phase_id, version, fields, released_by)
		VALUES (?, ?, ?, ?, NULLIF(?, 0))`, organizationID, id, version, string(encoded), accountID).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: insert träger phase version: %w", err)
	}
	if _, err := db.NewRaw(`UPDATE enrollment.traeger_phases SET released_version = ? WHERE organization_id = ? AND id = ?`,
		version, organizationID, id).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: release träger phase: %w", err)
	}
	return nil
}

// RecordTraegerPhaseOutcomes writes the outcome of each school's copy. A
// school outside the Träger is refused by the school trigger.
func (s *TraegerPhaseStore) RecordTraegerPhaseOutcomes(ctx context.Context, organizationID, id int64, outcomes []enrollment.TraegerCopyOutcome) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if len(outcomes) == 0 {
		return nil
	}
	return markTraegerSchoolError(recordTraegerOutcomes(ctx, db, organizationID, id, outcomes), "record träger phase outcomes")
}

// recordTraegerOutcomes upserts the outcomes of one Träger phase in one
// statement. A row a rollout adds through all_schools counts as selected; a
// rejected or stale outcome keeps the version applied before.
func recordTraegerOutcomes(ctx context.Context, db bun.IDB, organizationID, id int64, outcomes []enrollment.TraegerCopyOutcome) error {
	type outcomeRow struct {
		SchoolID       int64   `json:"school_id"`
		AppliedVersion *int    `json:"applied_version"`
		Outcome        string  `json:"outcome"`
		ErrorCode      *string `json:"error_code"`
	}
	rows := make([]outcomeRow, 0, len(outcomes))
	for _, outcome := range outcomes {
		row := outcomeRow{SchoolID: outcome.SchoolID, AppliedVersion: outcome.AppliedVersion, Outcome: outcome.Outcome}
		if outcome.ErrorCode != "" {
			code := outcome.ErrorCode
			row.ErrorCode = &code
		}
		rows = append(rows, row)
	}
	encoded, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	_, err = db.NewRaw(`INSERT INTO enrollment.traeger_phase_schools AS target (organization_id, traeger_phase_id, school_id, applied_version, outcome, error_code, recorded_at)
		SELECT ?, ?, outcome.school_id, outcome.applied_version, outcome.outcome, outcome.error_code, now()
		FROM jsonb_to_recordset(?::jsonb) AS outcome(school_id bigint, applied_version int, outcome text, error_code text)
		ON CONFLICT (organization_id, traeger_phase_id, school_id) DO UPDATE SET
			applied_version = CASE WHEN EXCLUDED.outcome IN ('applied', 'withdrawn') THEN EXCLUDED.applied_version ELSE target.applied_version END,
			outcome = EXCLUDED.outcome, error_code = EXCLUDED.error_code, recorded_at = EXCLUDED.recorded_at`,
		organizationID, id, string(encoded)).Exec(ctx)
	return err
}

// markTraegerSchoolError names a school outside the Träger; the database
// decides it, nothing is pre-checked.
func markTraegerSchoolError(err error, operation string) error {
	if err == nil {
		return nil
	}
	var postgresError pgdriver.Error
	if errors.As(err, &postgresError) && postgresError.IntegrityViolation() && postgresError.Field('n') == traegerCatalogSchoolConstraint {
		return fmt.Errorf("%w: %w", enrollment.ErrTraegerSchoolOutsideTraeger, err)
	}
	return fmt.Errorf("enrollment postgres: %s: %w", operation, err)
}

func affectedOne(result sql.Result) (bool, error) {
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("enrollment postgres: affected rows: %w", err)
	}
	return rows > 0, nil
}
