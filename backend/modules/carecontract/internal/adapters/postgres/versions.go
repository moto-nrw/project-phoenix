package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/modules/carecontract/internal/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The versions of a contract over time (#4681). The effective versions
// (wirksam, abgeloest) form one timeline; the migration 1.15.787 keeps them
// apart. A version holds from its effective date to the day before the next
// effective version starts, or to its own end.

const effectiveStatusSQL = `('` + carecontract.VersionEffective + `', '` + carecontract.VersionSuperseded + `')`

// validUntilSQL is the last day version v holds; NULL when it holds open
// ended or is no effective version.
const validUntilSQL = `CASE WHEN v.status IN ` + effectiveStatusSQL + ` THEN LEAST(
		(SELECT MIN(n.effective_from) FROM care_contract.contract_versions AS n
			WHERE n.organization_id = v.organization_id AND n.contract_id = v.contract_id
				AND n.status IN ` + effectiveStatusSQL + ` AND n.effective_from > v.effective_from) - 1,
		v.ends_on) END`

// versionsSQL reads every version of a contract with its services, oldest
// first.
// The last day a version holds is cut at the last day of a terminated
// contract (#4682), so the list shows where the contract ends.
const versionsSQL = `SELECT ` + versionColumns + `, v.source, ` + servicesSQL + ` AS services,
	CASE WHEN v.status IN ` + effectiveStatusSQL + ` THEN LEAST(` + validUntilSQL + `,
		(SELECT c.ends_on FROM care_contract.contracts AS c WHERE c.organization_id = v.organization_id AND c.id = v.contract_id))
	END AS valid_until
	FROM care_contract.contract_versions AS v
	WHERE v.organization_id = ? AND v.contract_id = ? ORDER BY v.version_number`

func (Store) Versions(ctx context.Context, organizationID, contractID int64) ([]carecontract.Version, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []versionRow
	if err := db.NewRaw(versionsSQL, organizationID, contractID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care contract postgres: read versions: %w", err)
	}
	return versionsOf(rows)
}

func versionsOf(rows []versionRow) ([]carecontract.Version, error) {
	versions := make([]carecontract.Version, len(rows))
	for i, row := range rows {
		version, err := row.version()
		if err != nil {
			return nil, err
		}
		versions[i] = version
	}
	return versions, nil
}

// AmendableContracts reads every active contract from an application (a
// contract from an import file follows no bookings, #4723) with its effective
// versions by effective date, the start of version 1, which a fixed term
// counts from even when a later change goes before it, and its next version
// number.
func (Store) AmendableContracts(ctx context.Context, organizationID int64) ([]ports.AmendableContract, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var heads []struct {
		ContractID     int64         `bun:"contract_id"`
		SchoolID       int64         `bun:"school_id"`
		RequestChildID int64         `bun:"request_child_id"`
		FirstFrom      calendar.Date `bun:"first_from,type:date"`
		NextNumber     int           `bun:"next_number"`
	}
	if err := db.NewRaw(`SELECT c.id AS contract_id, c.school_id, c.request_child_id,
			(SELECT f.effective_from FROM care_contract.contract_versions AS f
				WHERE f.organization_id = c.organization_id AND f.contract_id = c.id
					AND f.version_number = 1) AS first_from,
			(SELECT MAX(m.version_number) + 1 FROM care_contract.contract_versions AS m
				WHERE m.organization_id = c.organization_id AND m.contract_id = c.id) AS next_number
		FROM care_contract.contracts AS c
		JOIN care_contract.contract_versions AS v ON v.organization_id = c.organization_id AND v.contract_id = c.id
			AND v.status = ?
		WHERE c.organization_id = ? AND c.status = ? AND c.request_child_id IS NOT NULL
		ORDER BY c.school_id, c.id`, carecontract.VersionEffective, organizationID, carecontract.StatusActive).Scan(ctx, &heads); err != nil {
		return nil, fmt.Errorf("care contract postgres: read active contracts: %w", err)
	}
	if len(heads) == 0 {
		return []ports.AmendableContract{}, nil
	}
	contractIDs := make([]int64, len(heads))
	for i, head := range heads {
		contractIDs[i] = head.ContractID
	}
	timelines, err := effectiveTimelines(ctx, db, organizationID, contractIDs)
	if err != nil {
		return nil, err
	}
	result := make([]ports.AmendableContract, 0, len(heads))
	for _, head := range heads {
		timeline := timelines[head.ContractID]
		if len(timeline) == 0 {
			continue
		}
		result = append(result, ports.AmendableContract{
			ID: head.ContractID, Key: ports.ContractKey{SchoolID: head.SchoolID, RequestChildID: head.RequestChildID},
			FirstFrom: head.FirstFrom, NextNumber: head.NextNumber, Latest: timeline[len(timeline)-1], Versions: timeline,
		})
	}
	return result, nil
}

// effectiveTimelines reads the effective versions of the contracts by
// effective date, in one statement.
func effectiveTimelines(ctx context.Context, db bun.IDB, organizationID int64, ids []int64) (map[int64][]carecontract.Version, error) {
	var rows []periodRow
	if err := db.NewRaw(`SELECT v.contract_id, `+versionColumns+`, v.source, `+servicesSQL+` AS services, `+validUntilSQL+` AS valid_until
		FROM care_contract.contract_versions AS v
		WHERE v.organization_id = ? AND v.contract_id = ANY(?) AND v.status IN `+effectiveStatusSQL+`
		ORDER BY v.contract_id, v.effective_from`, organizationID, pgdialect.Array(ids)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care contract postgres: read effective versions: %w", err)
	}
	timelines := make(map[int64][]carecontract.Version, len(ids))
	for _, row := range rows {
		version, err := row.version()
		if err != nil {
			return nil, err
		}
		timelines[row.ContractID] = append(timelines[row.ContractID], version)
	}
	return timelines, nil
}

// InsertVersion stores a new version after its step on the timeline: the
// wirksam version it follows becomes abgeloest, or the version starting on
// its day becomes ersetzt, or, before a planned version, nothing changes. A
// stepped version that is no longer in its status is ErrVersionMoved, so a
// concurrent run adds nothing twice. The triggers of 1.15.787 refuse an
// overlap and a version in the past.
func (Store) InsertVersion(ctx context.Context, organizationID, contractID int64, step ports.VersionStep, version carecontract.Version) (int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	stepped, status := step.Supersede, carecontract.VersionSuperseded
	if step.Replace != 0 {
		stepped, status = step.Replace, carecontract.VersionReplaced
	}
	if stepped == 0 {
		return insertVersion(ctx, db, organizationID, contractID, version)
	}
	result, err := db.ExecContext(ctx, `UPDATE care_contract.contract_versions SET status = ?
		WHERE organization_id = ? AND contract_id = ? AND id = ? AND status = ?`,
		status, organizationID, contractID, stepped, step.Status)
	if err != nil {
		return 0, fmt.Errorf("care contract postgres: supersede version: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return 0, ports.ErrVersionMoved
	}
	return insertVersion(ctx, db, organizationID, contractID, version)
}

func insertVersion(ctx context.Context, db bun.IDB, organizationID, contractID int64, version carecontract.Version) (int64, error) {
	terms := version.Terms
	source := version.Source
	if source == "" {
		source = carecontract.SourceConclusion
	}
	var versionID int64
	if err := db.NewRaw(`INSERT INTO care_contract.contract_versions (organization_id, contract_id,
			version_number, status, effective_from, ends_on, child_first_name, child_last_name, child_birth_date,
			school_name, phase_name, price_cents, term_kind, term_months, notice_months, notice_to, special_termination,
			special_termination_note, statute_required, statute_title, statute_url, contract_partner, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		organizationID, contractID, version.Number, version.Status, version.EffectiveFrom, version.EndsOn,
		version.ChildFirstName, version.ChildLastName, version.ChildBirthDate, version.SchoolName, version.PhaseName,
		version.PriceCents,
		terms.TermKind, terms.TermMonths, terms.NoticeMonths, terms.NoticeTo, terms.SpecialTermination,
		terms.SpecialTerminationNote, terms.StatuteRequired, terms.StatuteTitle, terms.StatuteURL,
		terms.ContractPartner, source).Scan(ctx, &versionID); err != nil {
		return 0, fmt.Errorf("care contract postgres: insert version: %w", err)
	}
	if err := insertServices(ctx, db, organizationID, versionID, version.Services); err != nil {
		return 0, err
	}
	return versionID, nil
}

// periodRow is one version in force with its contract.
type periodRow struct {
	versionRow
	ContractID int64  `bun:"contract_id"`
	SchoolID   int64  `bun:"school_id"`
	StudentID  *int64 `bun:"student_id"`
}

// VersionsInForce reads the effective versions of the active, terminated
// and ended contracts that hold on a day between from and to, by contract
// and start. A terminated contract cuts its versions at its last day
// (#4682); a withdrawn one holds on no day.
func (Store) VersionsInForce(ctx context.Context, organizationID int64, from, to calendar.Date, contractIDs []int64) ([]carecontract.VersionPeriod, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []periodRow
	if err := db.NewRaw(`SELECT * FROM (
			SELECT c.id AS contract_id, c.school_id, c.student_id, `+versionColumns+`, v.source, `+servicesSQL+` AS services,
				LEAST(`+validUntilSQL+`, c.ends_on) AS valid_until
			FROM care_contract.contracts AS c
			JOIN care_contract.contract_versions AS v ON v.organization_id = c.organization_id AND v.contract_id = c.id
			WHERE c.organization_id = ? AND c.status IN (?, ?, ?) AND v.status IN `+effectiveStatusSQL+`
				AND (c.ends_on IS NULL OR v.effective_from <= c.ends_on)
				AND v.effective_from <= ? AND (cardinality(?::bigint[]) = 0 OR c.id = ANY(?::bigint[]))) AS t
		WHERE t.valid_until IS NULL OR t.valid_until >= ?
		ORDER BY t.contract_id, t.effective_from`,
		organizationID, carecontract.StatusActive, carecontract.StatusTerminated, carecontract.StatusEnded, to,
		pgdialect.Array(nonNilIDs(contractIDs)),
		pgdialect.Array(nonNilIDs(contractIDs)), from).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care contract postgres: read versions in force: %w", err)
	}
	periods := make([]carecontract.VersionPeriod, len(rows))
	for i, row := range rows {
		version, err := row.version()
		if err != nil {
			return nil, err
		}
		periods[i] = carecontract.VersionPeriod{ContractID: row.ContractID, SchoolID: row.SchoolID, StudentID: row.StudentID, Version: version}
	}
	return periods, nil
}

func nonNilIDs(ids []int64) []int64 {
	if ids == nil {
		return []int64{}
	}
	return ids
}

func (Store) VersionDocument(ctx context.Context, organizationID, versionID int64) (carecontract.File, bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return carecontract.File{}, false, err
	}
	var row documentRow
	err = db.NewRaw(`SELECT d.filename, d.pdf, d.withdrawal_notice FROM care_contract.contract_documents AS d
		WHERE d.organization_id = ? AND d.contract_version_id = ?`, organizationID, versionID).Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return carecontract.File{}, false, nil
	}
	file, err := row.file(err, carecontract.ErrVersionNotFound)
	return file, err == nil, err
}
