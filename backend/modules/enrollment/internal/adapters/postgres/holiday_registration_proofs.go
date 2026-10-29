package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Proofs of benefits of the public holiday registration (#5238). Every
// statement but the retention runs in the Träger transaction of the Träger
// it names and names it once more in its filter. Only the read of one opened
// proof selects the bytes.

const (
	insertHolidayProofQuery = `INSERT INTO enrollment.holiday_registration_proofs
			(organization_id, holiday_period_id, token_hash, filename, content_type, size_bytes, content)
		VALUES (?, ?, ?, ?, ?, ?, ?)`
	// claimHolidayProofsQuery binds only unclaimed proofs of the same period
	// that are younger than the unclaimed lifetime. Two submissions with the
	// same token serialize on the row lock; the second binds nothing.
	claimHolidayProofsQuery = `UPDATE enrollment.holiday_registration_proofs
		SET holiday_registration_id = ?, token_hash = NULL, claimed_at = statement_timestamp()
		WHERE organization_id = ? AND holiday_period_id = ? AND holiday_registration_id IS NULL
			AND token_hash IN (?)
			AND created_at > statement_timestamp() - (? * INTERVAL '1 second')`
	readHolidayProofsQuery = `SELECT id, filename, size_bytes, content_type, created_at
		FROM enrollment.holiday_registration_proofs
		WHERE organization_id = ? AND holiday_registration_id = ?
		ORDER BY created_at, id`
	readHolidayProofQuery = `SELECT filename, content_type, content
		FROM enrollment.holiday_registration_proofs
		WHERE organization_id = ? AND holiday_period_id = ? AND holiday_registration_id = ? AND id = ?`
	deleteUnclaimedHolidayProofsQuery = `DELETE FROM enrollment.holiday_registration_proofs
		WHERE holiday_registration_id IS NULL AND created_at < ?`
)

// InsertHolidayProof stores an unclaimed proof of the period.
func (s *HolidayRegistrationStore) InsertHolidayProof(ctx context.Context, organizationID, periodID int64, proof enrollment.HolidayProofUpload, tokenHash string) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(insertHolidayProofQuery, organizationID, periodID, tokenHash, proof.Filename,
		proof.ContentType, proof.SizeBytes, proof.Content).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: insert holiday proof: %w", err)
	}
	return nil
}

// ClaimHolidayProofs binds the listed unclaimed proofs of the period to the
// registration and reports how many it bound.
func (s *HolidayRegistrationStore) ClaimHolidayProofs(ctx context.Context, organizationID, periodID, registrationID int64, tokenHashes []string) (int, error) {
	if len(tokenHashes) == 0 {
		return 0, nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	claimed, err := affectedRows(db.NewRaw(claimHolidayProofsQuery, registrationID, organizationID, periodID,
		bun.List(tokenHashes), int64(enrollment.UnclaimedUploadLifetime.Seconds())).Exec(ctx))
	if err != nil {
		return 0, fmt.Errorf("enrollment postgres: claim holiday proofs: %w", err)
	}
	return claimed, nil
}

// ReadHolidayProofs reads the claimed proofs of a registration without
// their bytes.
func (s *HolidayRegistrationStore) ReadHolidayProofs(ctx context.Context, organizationID, registrationID int64) ([]enrollment.HolidayRegistrationProof, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID          int64     `bun:"id"`
		Filename    string    `bun:"filename"`
		SizeBytes   int64     `bun:"size_bytes"`
		ContentType string    `bun:"content_type"`
		CreatedAt   time.Time `bun:"created_at"`
	}
	if err := db.NewRaw(readHolidayProofsQuery, organizationID, registrationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read holiday proofs: %w", err)
	}
	proofs := make([]enrollment.HolidayRegistrationProof, len(rows))
	for i, row := range rows {
		proofs[i] = enrollment.HolidayRegistrationProof{
			ID: row.ID, Filename: row.Filename, SizeBytes: row.SizeBytes, ContentType: row.ContentType, UploadedAt: row.CreatedAt,
		}
	}
	return proofs, nil
}

// ReadHolidayProof reads one claimed proof of a registration of the period
// with its bytes.
func (s *HolidayRegistrationStore) ReadHolidayProof(ctx context.Context, organizationID, periodID, registrationID, proofID int64) (enrollment.HolidayProofFile, bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return enrollment.HolidayProofFile{}, false, err
	}
	var rows []struct {
		Filename    string `bun:"filename"`
		ContentType string `bun:"content_type"`
		Content     []byte `bun:"content"`
	}
	if err := db.NewRaw(readHolidayProofQuery, organizationID, periodID, registrationID, proofID).Scan(ctx, &rows); err != nil {
		return enrollment.HolidayProofFile{}, false, fmt.Errorf("enrollment postgres: read holiday proof: %w", err)
	}
	if len(rows) == 0 {
		return enrollment.HolidayProofFile{}, false, nil
	}
	return enrollment.HolidayProofFile(rows[0]), true, nil
}

// DeleteUnclaimedHolidayProofs deletes, across every Träger, the proofs no
// registration claimed that were uploaded before createdBefore. Only the
// administrative transaction of the hourly cleanup may delete across Träger.
func (s *HolidayRegistrationStore) DeleteUnclaimedHolidayProofs(ctx context.Context, createdBefore time.Time) (int, error) {
	if !tenant.IsAdminTx(ctx) {
		return 0, errors.New("enrollment postgres: the holiday proof retention requires the administrative transaction")
	}
	db, err := NewDatabase()(ctx)
	if err != nil {
		return 0, err
	}
	deleted, err := affectedRows(db.NewRaw(deleteUnclaimedHolidayProofsQuery, createdBefore).Exec(ctx))
	if err != nil {
		return 0, fmt.Errorf("enrollment postgres: delete unclaimed holiday proofs: %w", err)
	}
	return deleted, nil
}
