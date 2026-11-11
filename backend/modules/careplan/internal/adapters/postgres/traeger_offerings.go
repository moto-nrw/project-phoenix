package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Träger care offerings (#4704, ADR 0195), Care Plan's first Träger tables.
// They carry organization_id and a Träger policy (ADR 0105): every statement
// runs in the Träger transaction of the Träger it names and names it once
// more in its own filter. The school tables of this package refuse that
// transaction, because it carries no school.

// traegerOfferingPhaseConstraint is the composite key that keeps an
// offering in a phase of its own Träger.
const traegerOfferingPhaseConstraint = "fk_traeger_care_offerings_phase"

// TraegerOfferingStore persists the Träger care offerings.
type TraegerOfferingStore struct{}

// NewTraegerOfferingStore builds the store.
func NewTraegerOfferingStore() *TraegerOfferingStore { return &TraegerOfferingStore{} }

// traegerDB returns the Träger transaction of organizationID. A school,
// administrative or foreign Träger transaction is refused.
func traegerDB(ctx context.Context, organizationID int64) (bun.IDB, error) {
	if organizationID <= 0 || tenant.TraegerTransactionFromContext(ctx) != organizationID {
		return nil, errors.New("care plan postgres: the Träger transaction of this Träger is required")
	}
	raw, present := tenant.TransactionFromContext(ctx)
	if !present {
		return nil, errors.New("care plan postgres: transaction is required")
	}
	switch tx := raw.(type) {
	case bun.Tx:
		return tx, nil
	case *bun.Tx:
		if tx != nil {
			return *tx, nil
		}
	}
	return nil, fmt.Errorf("care plan postgres: unsupported transaction %T", raw)
}

type traegerOfferingRow struct {
	ID              int64           `bun:"id"`
	OrganizationID  int64           `bun:"organization_id"`
	TraegerPhaseID  int64           `bun:"traeger_phase_id"`
	Draft           json.RawMessage `bun:"draft"`
	ReleasedVersion *int            `bun:"released_version"`
	Released        json.RawMessage `bun:"released"`
	WithdrawnAt     *time.Time      `bun:"withdrawn_at"`
	UpdatedAt       time.Time       `bun:"updated_at"`
}

type traegerOfferingSchoolRow struct {
	OfferingID     int64      `bun:"offering_id"`
	SchoolID       int64      `bun:"school_id"`
	AppliedVersion *int       `bun:"applied_version"`
	Outcome        *string    `bun:"outcome"`
	ErrorCode      *string    `bun:"error_code"`
	RecordedAt     *time.Time `bun:"recorded_at"`
}

const traegerOfferingSelect = `SELECT offering.id, offering.organization_id, offering.traeger_phase_id, offering.draft,
	offering.released_version, version.fields AS released, offering.withdrawn_at, offering.updated_at
	FROM enrollment.traeger_care_offerings AS offering
	LEFT JOIN enrollment.traeger_care_offering_versions AS version
		ON version.organization_id = offering.organization_id AND version.traeger_care_offering_id = offering.id
		AND version.version = offering.released_version
	WHERE offering.organization_id = ?`

const (
	traegerOfferingsOfPhaseSelect = traegerOfferingSelect + ` AND offering.traeger_phase_id = ? ORDER BY offering.id`
	traegerOfferingByIDSelect     = traegerOfferingSelect + ` AND offering.id = ?`
)

// ListTraegerOfferings returns the offerings of one Träger phase with their
// schools in two statements.
func (s *TraegerOfferingStore) ListTraegerOfferings(ctx context.Context, organizationID, traegerPhaseID int64) ([]careplan.TraegerOffering, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []traegerOfferingRow
	if err := db.NewRaw(traegerOfferingsOfPhaseSelect, organizationID, traegerPhaseID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care plan postgres: read träger care offerings: %w", err)
	}
	return s.withSchools(ctx, db, organizationID, rows)
}

// TraegerOffering returns one offering of the Träger.
func (s *TraegerOfferingStore) TraegerOffering(ctx context.Context, organizationID, id int64) (careplan.TraegerOffering, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return careplan.TraegerOffering{}, err
	}
	var rows []traegerOfferingRow
	if err := db.NewRaw(traegerOfferingByIDSelect, organizationID, id).Scan(ctx, &rows); err != nil {
		return careplan.TraegerOffering{}, fmt.Errorf("care plan postgres: read träger care offering: %w", err)
	}
	offerings, err := s.withSchools(ctx, db, organizationID, rows)
	if err != nil {
		return careplan.TraegerOffering{}, err
	}
	if len(offerings) == 0 {
		return careplan.TraegerOffering{}, careplan.ErrTraegerOfferingNotFound
	}
	return offerings[0], nil
}

func (s *TraegerOfferingStore) withSchools(ctx context.Context, db bun.IDB, organizationID int64, rows []traegerOfferingRow) ([]careplan.TraegerOffering, error) {
	offerings := make([]careplan.TraegerOffering, 0, len(rows))
	if len(rows) == 0 {
		return offerings, nil
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	var schools []traegerOfferingSchoolRow
	if err := db.NewRaw(`SELECT traeger_care_offering_id AS offering_id, school_id, applied_version, outcome, error_code, recorded_at
		FROM enrollment.traeger_care_offering_schools WHERE organization_id = ? AND traeger_care_offering_id IN (?) ORDER BY school_id`,
		organizationID, bun.List(ids)).Scan(ctx, &schools); err != nil {
		return nil, fmt.Errorf("care plan postgres: read träger care offering schools: %w", err)
	}
	byOffering := map[int64][]careplan.TraegerOfferingSchool{}
	for _, school := range schools {
		byOffering[school.OfferingID] = append(byOffering[school.OfferingID], school.value())
	}
	for _, row := range rows {
		offering, err := row.value()
		if err != nil {
			return nil, err
		}
		offering.Schools = byOffering[row.ID]
		if offering.Schools == nil {
			offering.Schools = []careplan.TraegerOfferingSchool{}
		}
		offerings = append(offerings, offering)
	}
	return offerings, nil
}

func (row traegerOfferingSchoolRow) value() careplan.TraegerOfferingSchool {
	school := careplan.TraegerOfferingSchool{SchoolID: row.SchoolID, AppliedVersion: row.AppliedVersion, RecordedAt: row.RecordedAt}
	if row.Outcome != nil {
		school.Outcome = *row.Outcome
	}
	if row.ErrorCode != nil {
		school.ErrorCode = *row.ErrorCode
	}
	return school
}

func (row traegerOfferingRow) value() (careplan.TraegerOffering, error) {
	offering := careplan.TraegerOffering{
		ID: row.ID, OrganizationID: row.OrganizationID, TraegerPhaseID: row.TraegerPhaseID,
		ReleasedVersion: row.ReleasedVersion, WithdrawnAt: row.WithdrawnAt, UpdatedAt: row.UpdatedAt,
	}
	if err := json.Unmarshal(row.Draft, &offering.Draft); err != nil {
		return offering, fmt.Errorf("care plan postgres: decode träger care offering draft: %w", err)
	}
	if len(row.Released) > 0 {
		var released careplan.TraegerOfferingDraft
		if err := json.Unmarshal(row.Released, &released); err != nil {
			return offering, fmt.Errorf("care plan postgres: decode träger care offering version: %w", err)
		}
		offering.Released = &released
	}
	return offering, nil
}

// InsertTraegerOffering creates an offering of the Träger phase. A phase
// of another Träger fails the composite foreign key and reads as not found.
func (s *TraegerOfferingStore) InsertTraegerOffering(ctx context.Context, organizationID, traegerPhaseID int64, draft careplan.TraegerOfferingDraft) (int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	encoded, err := json.Marshal(draft)
	if err != nil {
		return 0, fmt.Errorf("care plan postgres: encode träger care offering: %w", err)
	}
	var id int64
	if err := db.NewRaw(`INSERT INTO enrollment.traeger_care_offerings (organization_id, traeger_phase_id, draft)
		VALUES (?, ?, ?) RETURNING id`, organizationID, traegerPhaseID, string(encoded)).Scan(ctx, &id); err != nil {
		var postgresError pgdriver.Error
		if errors.As(err, &postgresError) && postgresError.IntegrityViolation() && postgresError.Field('n') == traegerOfferingPhaseConstraint {
			return 0, careplan.ErrTraegerOfferingNotFound
		}
		return 0, fmt.Errorf("care plan postgres: insert träger care offering: %w", err)
	}
	return id, nil
}

// UpdateTraegerOfferingDraft replaces the draft of an offering that is not
// withdrawn; false reports that no such offering exists.
func (s *TraegerOfferingStore) UpdateTraegerOfferingDraft(ctx context.Context, organizationID, id int64, draft careplan.TraegerOfferingDraft) (bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return false, err
	}
	encoded, err := json.Marshal(draft)
	if err != nil {
		return false, fmt.Errorf("care plan postgres: encode träger care offering: %w", err)
	}
	result, err := db.NewRaw(`UPDATE enrollment.traeger_care_offerings SET draft = ?
		WHERE organization_id = ? AND id = ? AND withdrawn_at IS NULL`, string(encoded), organizationID, id).Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("care plan postgres: update träger care offering: %w", err)
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

// WithdrawTraegerOffering marks the offering withdrawn; false reports that
// no offering exists.
func (s *TraegerOfferingStore) WithdrawTraegerOffering(ctx context.Context, organizationID, id int64) (bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return false, err
	}
	result, err := db.NewRaw(`UPDATE enrollment.traeger_care_offerings SET withdrawn_at = COALESCE(withdrawn_at, now())
		WHERE organization_id = ? AND id = ?`, organizationID, id).Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("care plan postgres: withdraw träger care offering: %w", err)
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

// InsertTraegerOfferingVersions writes the given versions as released by
// accountID and makes each the released version of its offering, in two
// statements.
func (s *TraegerOfferingStore) InsertTraegerOfferingVersions(ctx context.Context, organizationID int64, versions map[int64]careplan.TraegerOffering, accountID int64) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		return nil
	}
	type versionRow struct {
		ID      int64                         `json:"id"`
		Version int                           `json:"version"`
		Fields  careplan.TraegerOfferingDraft `json:"fields"`
	}
	rows := make([]versionRow, 0, len(versions))
	for id, offering := range versions {
		rows = append(rows, versionRow{ID: id, Version: *offering.ReleasedVersion, Fields: offering.Draft})
	}
	encoded, err := json.Marshal(rows)
	if err != nil {
		return fmt.Errorf("care plan postgres: encode träger care offering versions: %w", err)
	}
	if _, err := db.NewRaw(`INSERT INTO enrollment.traeger_care_offering_versions (organization_id, traeger_care_offering_id, version, fields, released_by)
		SELECT ?, version.id, version.version, version.fields, NULLIF(?, 0)
		FROM jsonb_to_recordset(?::jsonb) AS version(id bigint, version int, fields jsonb)`,
		organizationID, accountID, string(encoded)).Exec(ctx); err != nil {
		return fmt.Errorf("care plan postgres: insert träger care offering versions: %w", err)
	}
	if _, err := db.NewRaw(`UPDATE enrollment.traeger_care_offerings AS offering SET released_version = version.version
		FROM jsonb_to_recordset(?::jsonb) AS version(id bigint, version int)
		WHERE offering.organization_id = ? AND offering.id = version.id`, string(encoded), organizationID).Exec(ctx); err != nil {
		return fmt.Errorf("care plan postgres: release träger care offerings: %w", err)
	}
	return nil
}

// RecordTraegerOfferingOutcomes writes the outcome of each school's copy in
// one statement. A school outside the Träger is refused by the trigger.
func (s *TraegerOfferingStore) RecordTraegerOfferingOutcomes(ctx context.Context, organizationID, id int64, outcomes []careplan.TraegerOfferingOutcome) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if len(outcomes) == 0 {
		return nil
	}
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
	if _, err := db.NewRaw(`INSERT INTO enrollment.traeger_care_offering_schools AS target
			(organization_id, traeger_care_offering_id, school_id, applied_version, outcome, error_code, recorded_at)
		SELECT ?, ?, outcome.school_id, outcome.applied_version, outcome.outcome, outcome.error_code, now()
		FROM jsonb_to_recordset(?::jsonb) AS outcome(school_id bigint, applied_version int, outcome text, error_code text)
		ON CONFLICT (organization_id, traeger_care_offering_id, school_id) DO UPDATE SET
			applied_version = CASE WHEN EXCLUDED.outcome IN ('applied', 'withdrawn') THEN EXCLUDED.applied_version ELSE target.applied_version END,
			outcome = EXCLUDED.outcome, error_code = EXCLUDED.error_code, recorded_at = EXCLUDED.recorded_at`,
		organizationID, id, string(encoded)).Exec(ctx); err != nil {
		return fmt.Errorf("care plan postgres: record träger care offering outcomes: %w", err)
	}
	return nil
}

// The copies of Träger offerings in one school (ADR 0195 section 4). These
// statements run in the school's tenant transaction and filter on its tenant.

type traegerOfferingCopyStateRow struct {
	SourceID int64  `bun:"traeger_source_id"`
	ID       int64  `bun:"id"`
	Version  int    `bun:"traeger_source_version"`
	Stamp    string `bun:"stamp"`
}

const (
	traegerOfferingCopyStatesQuery = `SELECT traeger_source_id, id, traeger_source_version,
		concat_ws('|', id, traeger_source_version, updated_at, is_active, array_to_string(traeger_deviations, ',')) AS stamp
		FROM enrollment.care_offerings WHERE tenant_id = ? AND traeger_source_id IN (?) ORDER BY id`
	traegerOfferingCopyStatesLockQuery = traegerOfferingCopyStatesQuery + ` FOR UPDATE`
)

// TraegerOfferingCopyStates reads the copies of sourceIDs; lock takes their
// row locks in ID order.
func (s *Store) TraegerOfferingCopyStates(ctx context.Context, sourceIDs []int64, lock bool) ([]careplan.TraegerOfferingCopyState, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "read träger offering copies")
	if err != nil {
		return nil, err
	}
	states := []careplan.TraegerOfferingCopyState{}
	if len(sourceIDs) == 0 {
		return states, nil
	}
	var rows []traegerOfferingCopyStateRow
	if lock {
		err = db.NewRaw(traegerOfferingCopyStatesLockQuery, tenantID, bun.List(sourceIDs)).Scan(ctx, &rows)
	} else {
		err = db.NewRaw(traegerOfferingCopyStatesQuery, tenantID, bun.List(sourceIDs)).Scan(ctx, &rows)
	}
	if err != nil {
		return nil, fmt.Errorf("care plan postgres: read träger offering copies: %w", err)
	}
	for _, row := range rows {
		states = append(states, careplan.TraegerOfferingCopyState{SourceID: row.SourceID, CopyID: row.ID, Version: row.Version, Stamp: row.Stamp})
	}
	return states, nil
}

// LinkTraegerOfferingCopy records the applied version and its values on the
// copy; the trigger recomputes the deviations from the row.
func (s *Store) LinkTraegerOfferingCopy(ctx context.Context, copyID, sourceID int64, version int, values map[string]any) error {
	db, tenantID, err := s.databaseForWrite(ctx, "link a träger offering copy")
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("care plan postgres: encode träger offering values: %w", err)
	}
	if _, err := db.NewRaw(`UPDATE enrollment.care_offerings SET traeger_source_id = ?, traeger_source_version = ?,
		traeger_values = ?::jsonb, updated_at = NOW() WHERE tenant_id = ? AND id = ?`,
		sourceID, version, string(encoded), tenantID, copyID).Exec(ctx); err != nil {
		return fmt.Errorf("care plan postgres: link träger offering copy: %w", err)
	}
	return nil
}

// ReleaseTraegerOfferingCopy deactivates the copy and drops its link: it
// stays an ordinary, inactive offering of the school.
func (s *Store) ReleaseTraegerOfferingCopy(ctx context.Context, copyID int64) error {
	db, tenantID, err := s.databaseForWrite(ctx, "release a träger offering copy")
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(`UPDATE enrollment.care_offerings SET traeger_source_id = NULL, is_active = false, updated_at = NOW()
		WHERE tenant_id = ? AND id = ?`, tenantID, copyID).Exec(ctx); err != nil {
		return fmt.Errorf("care plan postgres: release träger offering copy: %w", err)
	}
	return nil
}
