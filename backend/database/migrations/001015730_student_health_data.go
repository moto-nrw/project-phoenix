package migrations

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/uptrace/bun"
)

const studentHealthDataVersion = "1.15.730"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentHealthDataVersion,
		Description: "Gesundheitsfeld der Kinder verschlüsselt speichern (#5472)",
		DependsOn: []string{
			traegerLeadershipRolesVersion,    // the last migration before this one
			holidayHealthDataVersion,         // the key ring HEALTH_DATA_KEYS
			studentOwnerStorageExpandVersion, // users.student_care_profiles
		},
		Precondition: studentHealthDataPrecondition,
	})
	Migrations.MustRegister(studentHealthDataUp, studentHealthDataDown)
}

// The health field of a child (#5472) leaves the plaintext column
// users.student_care_profiles.health_info for health_data, sealed with
// AES-256-GCM by Security Runtime's field cipher under HEALTH_DATA_KEYS, the
// key ring of #4722. Each value is bound to its school and care row
// (membership), so a value copied into another row does not open. The
// context below is the one the student directory projection seals and opens
// with (studentdirectoryview.StudentHealthContext).
//
// Down restores the plaintext column and opens every sealed value into it;
// it needs the key ring as soon as one value is sealed.
const (
	studentHealthDataAddSQL = `
ALTER TABLE users.student_care_profiles
	ADD COLUMN health_data BYTEA,
	ADD CONSTRAINT chk_student_care_profiles_health_data CHECK (octet_length(health_data) <= 1048576);
COMMENT ON COLUMN users.student_care_profiles.health_data IS
	'Sealed health information of the child (#5472); never plaintext.';
`
	studentHealthDataDropPlaintextSQL = `ALTER TABLE users.student_care_profiles DROP COLUMN health_info;`
	studentHealthDataRestoreSQL       = `ALTER TABLE users.student_care_profiles ADD COLUMN health_info TEXT;`
	studentHealthDataDropSealedSQL    = `ALTER TABLE users.student_care_profiles
	DROP CONSTRAINT chk_student_care_profiles_health_data,
	DROP COLUMN health_data;`
	studentHealthToSealSQL = `SELECT EXISTS (SELECT 1 FROM users.student_care_profiles WHERE health_info IS NOT NULL)`
)

func studentHealthContext(tenantID, membershipID int64) []byte {
	return fmt.Appendf(nil, "users.student_care_profiles.health_data:%d:%d", tenantID, membershipID)
}

// studentHealthCipher reads the key ring from the environment.
func studentHealthCipher() (holidayFieldCipher, error) {
	if strings.TrimSpace(os.Getenv(healthDataKeysEnv)) == "" {
		return nil, fmt.Errorf("%s environment variable is required to seal the health information of the children (#5472)", healthDataKeysEnv)
	}
	return holidayHealthCipher()
}

// studentHealthDataPrecondition refuses before the release when a child has
// a health field to seal and no key to seal it with.
func studentHealthDataPrecondition(ctx context.Context, db *bun.DB) error {
	if migrationPending(ctx, studentOwnerStorageExpandVersion) {
		return nil
	}
	var pending bool
	if err := db.NewRaw(studentHealthToSealSQL).Scan(ctx, &pending); err != nil {
		return fmt.Errorf("check student health information to seal: %w", err)
	}
	if !pending {
		return nil
	}
	_, err := studentHealthCipher()
	return err
}

func studentHealthDataUp(ctx context.Context, db *bun.DB) error {
	cipher, err := optionalHolidayHealthCipher()
	if err != nil {
		return err
	}
	return studentHealthDataUpWith(ctx, db, cipher)
}

// studentHealthDataUpWith adds the sealed column, seals every health field
// with cipher and drops the plaintext, in one transaction. A nil cipher is
// allowed only without anything to seal.
func studentHealthDataUpWith(ctx context.Context, db *bun.DB, cipher holidayFieldCipher) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, studentHealthDataAddSQL); err != nil {
			return fmt.Errorf("add sealed student health column: %w", err)
		}
		var rows []studentHealthRow
		if err := tx.NewRaw(`SELECT tenant_id, membership_id, health_info FROM users.student_care_profiles
			WHERE health_info IS NOT NULL`).Scan(ctx, &rows); err != nil {
			return fmt.Errorf("read student health information to seal: %w", err)
		}
		for _, row := range rows {
			if cipher == nil {
				return fmt.Errorf("student health information to seal without %s", healthDataKeysEnv)
			}
			sealed, err := cipher.Seal([]byte(*row.HealthInfo), studentHealthContext(row.TenantID, row.MembershipID))
			if err != nil {
				return fmt.Errorf("seal student health of membership %d: %w", row.MembershipID, err)
			}
			if _, err := tx.NewRaw(`UPDATE users.student_care_profiles SET health_data = ? WHERE membership_id = ?`,
				sealed, row.MembershipID).Exec(ctx); err != nil {
				return fmt.Errorf("store sealed student health of membership %d: %w", row.MembershipID, err)
			}
		}
		if _, err := tx.ExecContext(ctx, studentHealthDataDropPlaintextSQL); err != nil {
			return fmt.Errorf("drop plaintext student health column: %w", err)
		}
		return nil
	})
}

type studentHealthRow struct {
	TenantID     int64   `bun:"tenant_id"`
	MembershipID int64   `bun:"membership_id"`
	HealthInfo   *string `bun:"health_info"`
	HealthData   []byte  `bun:"health_data"`
}

func studentHealthDataDown(ctx context.Context, db *bun.DB) error {
	cipher, err := optionalHolidayHealthCipher()
	if err != nil {
		return err
	}
	return studentHealthDataDownWith(ctx, db, cipher)
}

// studentHealthDataDownWith restores the plaintext column, opens every
// sealed value into it and drops the sealed column.
func studentHealthDataDownWith(ctx context.Context, db *bun.DB, cipher holidayFieldCipher) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, studentHealthDataRestoreSQL); err != nil {
			return fmt.Errorf("restore plaintext student health column: %w", err)
		}
		var rows []studentHealthRow
		if err := tx.NewRaw(`SELECT tenant_id, membership_id, health_data FROM users.student_care_profiles
			WHERE health_data IS NOT NULL`).Scan(ctx, &rows); err != nil {
			return fmt.Errorf("read sealed student health information: %w", err)
		}
		for _, row := range rows {
			if cipher == nil {
				return fmt.Errorf("sealed student health information to open without %s", healthDataKeysEnv)
			}
			plaintext, err := cipher.Open(row.HealthData, studentHealthContext(row.TenantID, row.MembershipID))
			if err != nil {
				return fmt.Errorf("open student health of membership %d: %w", row.MembershipID, err)
			}
			if _, err := tx.NewRaw(`UPDATE users.student_care_profiles SET health_info = ? WHERE membership_id = ?`,
				string(plaintext), row.MembershipID).Exec(ctx); err != nil {
				return fmt.Errorf("restore student health of membership %d: %w", row.MembershipID, err)
			}
		}
		if _, err := tx.ExecContext(ctx, studentHealthDataDropSealedSQL); err != nil {
			return fmt.Errorf("drop sealed student health column: %w", err)
		}
		return nil
	})
}
