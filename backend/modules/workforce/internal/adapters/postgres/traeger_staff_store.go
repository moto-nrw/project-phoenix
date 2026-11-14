package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

// TraegerStaffStore persists the Personalstamm of ADR 0089 (#4705) in the
// schema workforce. Each method names the transaction it joins: the Träger
// transaction of the record's Träger (ADR 0105), the tenant transaction of
// one school for that school's links and bank rows, or the administrative
// transaction of the moto team for the release. RLS holds each boundary;
// the guards below refuse a call in the wrong one before any SQL runs.
type TraegerStaffStore struct{}

// NewTraegerStaffStore builds the store.
func NewTraegerStaffStore() *TraegerStaffStore { return &TraegerStaffStore{} }

const staffRecordPersonnelNumberIndex = "uq_workforce_staff_records_personnel_number"

// traegerStaffDB returns the Träger transaction of organizationID.
func traegerStaffDB(ctx context.Context, organizationID int64) (bun.IDB, error) {
	if organizationID <= 0 || tenant.TraegerTransactionFromContext(ctx) != organizationID {
		return nil, errors.New("workforce postgres: the Träger transaction of this Träger is required")
	}
	return tenant.CurrentTransaction[bun.Tx](ctx)
}

type staffReleaseRow struct {
	OrganizationID       int64     `bun:"organization_id"`
	Representation       string    `bun:"representation"`
	ReleasedByOperatorID int64     `bun:"released_by_operator_id"`
	ReleasedAt           time.Time `bun:"released_at"`
}

func (r staffReleaseRow) toDomain() domain.TraegerStaffRelease {
	return domain.TraegerStaffRelease(r)
}

type staffRecordRow struct {
	ID                         int64          `bun:"id"`
	OrganizationID             int64          `bun:"organization_id"`
	FirstName                  string         `bun:"first_name"`
	LastName                   string         `bun:"last_name"`
	Birthday                   *calendar.Date `bun:"birthday,type:date"`
	PersonnelNumber            *string        `bun:"personnel_number"`
	IBANSealed                 []byte         `bun:"iban_sealed"`
	IBANMasked                 *string        `bun:"iban_masked"`
	TaxIDSealed                []byte         `bun:"tax_id_sealed"`
	SocialSecurityNumberSealed []byte         `bun:"social_security_number_sealed"`
}

func (r staffRecordRow) toDomain() domain.TraegerStaffRecord {
	record := domain.TraegerStaffRecord{
		ID: r.ID, OrganizationID: r.OrganizationID, FirstName: r.FirstName, LastName: r.LastName,
		PersonnelNumber: r.PersonnelNumber, IBANSealed: r.IBANSealed, IBANMasked: r.IBANMasked,
		TaxIDSealed: r.TaxIDSealed, SocialSecurityNumberSealed: r.SocialSecurityNumberSealed,
	}
	if r.Birthday != nil {
		record.Birthday = r.Birthday.String()
	}
	return record
}

type staffRecordLinkRow struct {
	StaffRecordID int64     `bun:"staff_record_id"`
	SchoolID      int64     `bun:"school_id"`
	StaffID       int64     `bun:"membership_id"`
	LinkedAt      time.Time `bun:"linked_at"`
}

const staffRecordColumns = `id, organization_id, first_name, last_name, birthday, personnel_number,
	iban_sealed, iban_masked, tax_id_sealed, social_security_number_sealed`

const (
	findStaffRecordQuery = `SELECT ` + staffRecordColumns + ` FROM workforce.staff_records WHERE organization_id = ? AND id = ?`
	lockStaffRecordQuery = findStaffRecordQuery + ` FOR UPDATE`
)

// TraegerStaffRelease returns the release of the Träger, nil without one,
// in its Träger transaction.
func (s *TraegerStaffStore) TraegerStaffRelease(ctx context.Context, organizationID int64) (*domain.TraegerStaffRelease, error) {
	db, err := traegerStaffDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	return findStaffRelease(ctx, db, organizationID)
}

func findStaffRelease(ctx context.Context, db bun.IDB, organizationID int64) (*domain.TraegerStaffRelease, error) {
	var rows []staffReleaseRow
	if err := db.NewRaw(`SELECT organization_id, representation, released_by_operator_id, released_at
		FROM workforce.staff_releases WHERE organization_id = ?`, organizationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("workforce postgres: read staff release: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	release := rows[0].toDomain()
	return &release, nil
}

// ListStaffRecords returns every record of the Träger with its links, by
// name, in two statements.
func (s *TraegerStaffStore) ListStaffRecords(ctx context.Context, organizationID int64) ([]domain.TraegerStaffRecord, error) {
	db, err := traegerStaffDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []staffRecordRow
	if err := db.NewRaw(`SELECT `+staffRecordColumns+` FROM workforce.staff_records
		WHERE organization_id = ? ORDER BY lower(last_name), lower(first_name), id`, organizationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("workforce postgres: list staff records: %w", err)
	}
	return withStaffLinks(ctx, db, organizationID, rows)
}

// FindStaffRecord returns one record with its links; forUpdate locks the
// record until the transaction ends. A record of another Träger is not found.
func (s *TraegerStaffStore) FindStaffRecord(ctx context.Context, organizationID, recordID int64, forUpdate bool) (domain.TraegerStaffRecord, error) {
	db, err := traegerStaffDB(ctx, organizationID)
	if err != nil {
		return domain.TraegerStaffRecord{}, err
	}
	query := findStaffRecordQuery
	if forUpdate {
		query = lockStaffRecordQuery
	}
	var rows []staffRecordRow
	if err := db.NewRaw(query, organizationID, recordID).Scan(ctx, &rows); err != nil {
		return domain.TraegerStaffRecord{}, fmt.Errorf("workforce postgres: read staff record: %w", err)
	}
	if len(rows) == 0 {
		return domain.TraegerStaffRecord{}, domain.ErrTraegerStaffRecordMissing
	}
	records, err := withStaffLinks(ctx, db, organizationID, rows)
	if err != nil {
		return domain.TraegerStaffRecord{}, err
	}
	return records[0], nil
}

func withStaffLinks(ctx context.Context, db bun.IDB, organizationID int64, rows []staffRecordRow) ([]domain.TraegerStaffRecord, error) {
	records := make([]domain.TraegerStaffRecord, len(rows))
	index := make(map[int64]int, len(rows))
	ids := make([]int64, len(rows))
	for i, row := range rows {
		records[i] = row.toDomain()
		index[row.ID] = i
		ids[i] = row.ID
	}
	if len(ids) == 0 {
		return records, nil
	}
	var links []staffRecordLinkRow
	if err := db.NewRaw(`SELECT staff_record_id, school_id, membership_id, linked_at FROM workforce.staff_record_links
		WHERE organization_id = ? AND staff_record_id = ANY(?) ORDER BY linked_at, id`, organizationID, pgdialect.Array(ids)).Scan(ctx, &links); err != nil {
		return nil, fmt.Errorf("workforce postgres: read staff record links: %w", err)
	}
	for _, link := range links {
		i := index[link.StaffRecordID]
		records[i].Links = append(records[i].Links, domain.TraegerStaffLink{SchoolID: link.SchoolID, StaffID: link.StaffID, LinkedAt: link.LinkedAt})
	}
	return records, nil
}

// LinkedEmployments returns which of the employments already belong to a
// record of the Träger, as school and staff pairs.
func (s *TraegerStaffStore) LinkedEmployments(ctx context.Context, organizationID int64, schoolIDs, staffIDs []int64) ([]domain.TraegerStaffLink, error) {
	db, err := traegerStaffDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []staffRecordLinkRow
	if err := db.NewRaw(`SELECT link.staff_record_id, link.school_id, link.membership_id, link.linked_at
		FROM workforce.staff_record_links AS link
		JOIN unnest(?::bigint[], ?::bigint[]) AS wanted (school_id, membership_id)
		  ON wanted.school_id = link.school_id AND wanted.membership_id = link.membership_id
		WHERE link.organization_id = ?`, pgdialect.Array(schoolIDs), pgdialect.Array(staffIDs), organizationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("workforce postgres: read linked employments: %w", err)
	}
	links := make([]domain.TraegerStaffLink, 0, len(rows))
	for _, row := range rows {
		links = append(links, domain.TraegerStaffLink{SchoolID: row.SchoolID, StaffID: row.StaffID, LinkedAt: row.LinkedAt})
	}
	return links, nil
}

// SaveStaffRecord inserts a record without an ID or updates the names,
// birthday, number and bank values of an existing one. A number another
// record of the Träger holds fails with domain.ErrStaffPersonnelNumberTaken.
func (s *TraegerStaffStore) SaveStaffRecord(ctx context.Context, record domain.TraegerStaffRecord) (int64, error) {
	db, err := traegerStaffDB(ctx, record.OrganizationID)
	if err != nil {
		return 0, err
	}
	var birthday *calendar.Date
	if record.Birthday != "" {
		day, err := calendar.ParseDate(record.Birthday)
		if err != nil {
			return 0, fmt.Errorf("workforce postgres: staff record birthday: %w", err)
		}
		birthday = &day
	}
	id := record.ID
	if id == 0 {
		err = db.NewRaw(`INSERT INTO workforce.staff_records (organization_id, first_name, last_name, birthday, personnel_number,
			iban_sealed, iban_masked, tax_id_sealed, social_security_number_sealed)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			record.OrganizationID, record.FirstName, record.LastName, birthday, record.PersonnelNumber,
			record.IBANSealed, record.IBANMasked, record.TaxIDSealed, record.SocialSecurityNumberSealed).Scan(ctx, &id)
	} else {
		_, err = db.ExecContext(ctx, `UPDATE workforce.staff_records SET first_name = ?, last_name = ?, birthday = ?, personnel_number = ?,
			iban_sealed = ?, iban_masked = ?, tax_id_sealed = ?, social_security_number_sealed = ?
			WHERE organization_id = ? AND id = ?`,
			record.FirstName, record.LastName, birthday, record.PersonnelNumber,
			record.IBANSealed, record.IBANMasked, record.TaxIDSealed, record.SocialSecurityNumberSealed,
			record.OrganizationID, record.ID)
	}
	if isUniqueViolationOn(err, staffRecordPersonnelNumberIndex) {
		return 0, domain.ErrStaffPersonnelNumberTaken
	}
	if err != nil {
		return 0, fmt.Errorf("workforce postgres: save staff record: %w", err)
	}
	return id, nil
}

// SealStaffRecord stores the sealed bank and tax values of a saved record.
func (s *TraegerStaffStore) SealStaffRecord(ctx context.Context, record domain.TraegerStaffRecord) error {
	db, err := traegerStaffDB(ctx, record.OrganizationID)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `UPDATE workforce.staff_records SET iban_sealed = ?, iban_masked = ?, tax_id_sealed = ?, social_security_number_sealed = ?
		WHERE organization_id = ? AND id = ?`, record.IBANSealed, record.IBANMasked, record.TaxIDSealed, record.SocialSecurityNumberSealed,
		record.OrganizationID, record.ID); err != nil {
		return fmt.Errorf("workforce postgres: seal staff record: %w", err)
	}
	return nil
}

// InsertStaffLinks adds Einsatzorte to a record in one statement. An
// employment already linked, or a second one of a school, fails with
// domain.ErrTraegerStaffEmploymentLinked.
func (s *TraegerStaffStore) InsertStaffLinks(ctx context.Context, organizationID, recordID, accountID int64, links []domain.TraegerStaffLink) error {
	db, err := traegerStaffDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if len(links) == 0 {
		return nil
	}
	schools, staff := make([]int64, len(links)), make([]int64, len(links))
	for i, link := range links {
		schools[i], staff[i] = link.SchoolID, link.StaffID
	}
	_, err = db.ExecContext(ctx, `INSERT INTO workforce.staff_record_links (organization_id, staff_record_id, school_id, membership_id, linked_by_account_id)
		SELECT ?, ?, link.school_id, link.membership_id, ? FROM unnest(?::bigint[], ?::bigint[]) AS link (school_id, membership_id)`,
		organizationID, recordID, accountID, pgdialect.Array(schools), pgdialect.Array(staff))
	if pgErr, ok := errors.AsType[pgdriver.Error](err); ok && pgErr.Field('C') == "23505" {
		return domain.ErrTraegerStaffEmploymentLinked
	}
	if err != nil {
		return fmt.Errorf("workforce postgres: link staff record: %w", err)
	}
	return nil
}

// AppendStaffChanges writes trail entries in one statement.
func (s *TraegerStaffStore) AppendStaffChanges(ctx context.Context, organizationID int64, changes []domain.TraegerStaffChange) error {
	db, err := traegerStaffDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if len(changes) == 0 {
		return nil
	}
	type changeRow struct {
		bun.BaseModel  `bun:"table:workforce.staff_record_changes"`
		OrganizationID int64   `bun:"organization_id"`
		StaffRecordID  int64   `bun:"staff_record_id"`
		Kind           string  `bun:"kind"`
		Field          string  `bun:"field"`
		ValueText      *string `bun:"value_text"`
		ValueSealed    []byte  `bun:"value_sealed"`
		ValueMasked    *string `bun:"value_masked"`
		SchoolID       *int64  `bun:"school_id"`
		AccountID      int64   `bun:"account_id"`
	}
	rows := make([]changeRow, len(changes))
	for i, change := range changes {
		rows[i] = changeRow{
			OrganizationID: organizationID, StaffRecordID: change.RecordID, Kind: change.Kind, Field: change.Field,
			ValueText: change.ValueText, ValueSealed: change.ValueSealed, ValueMasked: change.ValueMasked,
			SchoolID: change.SchoolID, AccountID: change.AccountID,
		}
	}
	if _, err := db.NewInsert().Model(&rows).Exec(ctx); err != nil {
		return fmt.Errorf("workforce postgres: append staff record changes: %w", err)
	}
	return nil
}

// ListStaffChanges returns the newest limit trail entries of a record.
func (s *TraegerStaffStore) ListStaffChanges(ctx context.Context, organizationID, recordID int64, limit int) ([]domain.TraegerStaffChange, error) {
	db, err := traegerStaffDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Kind        string    `bun:"kind"`
		Field       string    `bun:"field"`
		ValueText   *string   `bun:"value_text"`
		ValueMasked *string   `bun:"value_masked"`
		SchoolID    *int64    `bun:"school_id"`
		AccountID   int64     `bun:"account_id"`
		OccurredAt  time.Time `bun:"occurred_at"`
	}
	if err := db.NewRaw(`SELECT kind, field, value_text, value_masked, school_id, account_id, occurred_at
		FROM workforce.staff_record_changes WHERE organization_id = ? AND staff_record_id = ?
		ORDER BY occurred_at DESC, id DESC LIMIT ?`, organizationID, recordID, limit).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("workforce postgres: list staff record changes: %w", err)
	}
	changes := make([]domain.TraegerStaffChange, 0, len(rows))
	for _, row := range rows {
		changes = append(changes, domain.TraegerStaffChange{
			RecordID: recordID, Kind: row.Kind, Field: row.Field, ValueText: row.ValueText, ValueMasked: row.ValueMasked,
			SchoolID: row.SchoolID, AccountID: row.AccountID, OccurredAt: row.OccurredAt,
		})
	}
	return changes, nil
}

// DeleteStaffRecord removes a record with its links and trail.
func (s *TraegerStaffStore) DeleteStaffRecord(ctx context.Context, organizationID, recordID int64) error {
	db, err := traegerStaffDB(ctx, organizationID)
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, `DELETE FROM workforce.staff_records WHERE organization_id = ? AND id = ?`, organizationID, recordID)
	if err != nil {
		return fmt.Errorf("workforce postgres: delete staff record: %w", err)
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return domain.ErrTraegerStaffRecordMissing
	}
	return nil
}
