package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Master data of a Träger (#4653, ADR 0112). Its tables carry
// organization_id and the Träger policy on app.current_org_id (ADR 0086
// section 5, ADR 0105): every statement here runs in the Träger transaction
// of the Träger it names, and names it once more in its own filter.

// traegerAuditRetention is how long the master data log keeps an entry, the
// three years of the settings audit (#4044, ADR 0107 point 6).
const traegerAuditRetention = "3 years"

// TraegerMasterDataStore persists the master data in the Träger transaction.
type TraegerMasterDataStore struct{}

// NewTraegerMasterDataStore builds the store.
func NewTraegerMasterDataStore() *TraegerMasterDataStore { return &TraegerMasterDataStore{} }

// traegerDB returns the Träger transaction of organizationID. A school,
// administrative or foreign Träger transaction is refused.
func traegerDB(ctx context.Context, organizationID int64) (bun.IDB, error) {
	if organizationID <= 0 || tenant.TraegerTransactionFromContext(ctx) != organizationID {
		return nil, errors.New("organization tenancy postgres: the Träger transaction of this Träger is required")
	}
	return NewDatabase()(ctx)
}

type traegerProfileRow struct {
	ContactName     string     `bun:"contact_name"`
	ContactPhone    string     `bun:"contact_phone"`
	ContactEmail    string     `bun:"contact_email"`
	WithdrawalPhone string     `bun:"withdrawal_phone"`
	WithdrawalEmail string     `bun:"withdrawal_email"`
	LogoContentType *string    `bun:"logo_content_type"`
	LogoWidth       *int       `bun:"logo_width"`
	LogoHeight      *int       `bun:"logo_height"`
	LogoSize        *int       `bun:"logo_size"`
	LogoUpdatedAt   *time.Time `bun:"logo_updated_at"`
}

func (r traegerProfileRow) toDomain() organizationtenancy.TraegerProfile {
	profile := organizationtenancy.TraegerProfile{
		Contact:    organizationtenancy.TraegerContact{Name: r.ContactName, Phone: r.ContactPhone, Email: r.ContactEmail},
		Withdrawal: organizationtenancy.TraegerWithdrawalContact{Phone: r.WithdrawalPhone, Email: r.WithdrawalEmail},
	}
	if r.LogoContentType == nil || r.LogoWidth == nil || r.LogoHeight == nil || r.LogoSize == nil || r.LogoUpdatedAt == nil {
		return profile
	}
	profile.Logo = &organizationtenancy.TraegerLogo{
		ContentType: *r.LogoContentType, Width: *r.LogoWidth, Height: *r.LogoHeight, SizeBytes: *r.LogoSize, UpdatedAt: *r.LogoUpdatedAt,
	}
	return profile
}

type traegerBankAccountRow struct {
	ID            int64     `bun:"id"`
	Label         string    `bun:"label"`
	AccountHolder string    `bun:"account_holder"`
	IBAN          string    `bun:"iban"`
	BIC           string    `bun:"bic"`
	BankName      string    `bun:"bank_name"`
	IsDefault     bool      `bun:"is_default"`
	UpdatedAt     time.Time `bun:"updated_at"`
}

func (r traegerBankAccountRow) toDomain() organizationtenancy.TraegerBankAccount {
	return organizationtenancy.TraegerBankAccount(r)
}

const (
	traegerProfileColumns = `contact_name, contact_phone, contact_email, withdrawal_phone, withdrawal_email, logo_content_type, logo_width, logo_height,
		octet_length(logo) AS logo_size, logo_updated_at`
	findTraegerProfileQuery = `SELECT ` + traegerProfileColumns + ` FROM platform.traeger_profiles
		WHERE organization_id = ?`
	// findTraegerMasterDataQuery joins the Träger's own profile and bank
	// accounts onto one base row, in the bank account order of
	// listTraegerBankAccountsQuery.
	findTraegerMasterDataQuery = `SELECT profile.organization_id IS NOT NULL AS has_profile,
		profile.contact_name, profile.contact_phone, profile.contact_email,
		profile.withdrawal_phone, profile.withdrawal_email, profile.logo_content_type,
		profile.logo_width, profile.logo_height, octet_length(profile.logo) AS logo_size, profile.logo_updated_at,
		account.id AS bank_account_id, account.id AS bank_id, account.label AS bank_label, account.account_holder AS bank_account_holder,
		account.iban AS bank_iban, account.bic AS bank_bic, account.bank_name AS bank_bank_name,
		account.is_default AS bank_is_default, account.updated_at AS bank_updated_at
		FROM (SELECT ?::bigint AS organization_id) AS traeger
		LEFT JOIN platform.traeger_profiles AS profile ON profile.organization_id = traeger.organization_id
		LEFT JOIN platform.traeger_bank_accounts AS account ON account.organization_id = traeger.organization_id
		ORDER BY account.is_default DESC, account.id`
	ensureTraegerProfileQuery = `INSERT INTO platform.traeger_profiles (organization_id) VALUES (?)
		ON CONFLICT (organization_id) DO NOTHING`
	lockTraegerProfileQuery = findTraegerProfileQuery + ` FOR UPDATE`
	saveTraegerContactQuery = `UPDATE platform.traeger_profiles
		SET contact_name = ?, contact_phone = ?, contact_email = ?, updated_by = ?
		WHERE organization_id = ?`
	saveTraegerWithdrawalContactQuery = `UPDATE platform.traeger_profiles
		SET withdrawal_phone = ?, withdrawal_email = ?, updated_by = ?
		WHERE organization_id = ?`
	saveTraegerLogoQuery = `UPDATE platform.traeger_profiles
		SET logo = ?, logo_content_type = ?, logo_width = ?, logo_height = ?, logo_updated_at = CURRENT_TIMESTAMP, updated_by = ?
		WHERE organization_id = ?
		RETURNING ` + traegerProfileColumns
	clearTraegerLogoQuery = `UPDATE platform.traeger_profiles
		SET logo = NULL, logo_content_type = NULL, logo_width = NULL, logo_height = NULL, logo_updated_at = NULL, updated_by = ?
		WHERE organization_id = ?`
	traegerLogoImageQuery = `SELECT logo_content_type, logo, logo_updated_at FROM platform.traeger_profiles
		WHERE organization_id = ? AND logo IS NOT NULL`

	traegerBankAccountColumns    = `id, label, account_holder, iban, bic, bank_name, is_default, updated_at`
	listTraegerBankAccountsQuery = `SELECT ` + traegerBankAccountColumns + ` FROM platform.traeger_bank_accounts
		WHERE organization_id = ?
		ORDER BY is_default DESC, id`
	insertTraegerBankAccountQuery = `INSERT INTO platform.traeger_bank_accounts
		(organization_id, label, account_holder, iban, bic, bank_name, is_default, created_by, updated_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING ` + traegerBankAccountColumns
	updateTraegerBankAccountQuery = `UPDATE platform.traeger_bank_accounts
		SET label = ?, account_holder = ?, iban = ?, bic = ?, bank_name = ?, is_default = ?, updated_by = ?
		WHERE organization_id = ? AND id = ?
		RETURNING ` + traegerBankAccountColumns
	clearDefaultTraegerBankAccountQuery = `UPDATE platform.traeger_bank_accounts SET is_default = false
		WHERE organization_id = ? AND is_default`
	deleteTraegerBankAccountQuery = `DELETE FROM platform.traeger_bank_accounts WHERE organization_id = ? AND id = ?`

	pruneTraegerMasterDataAuditQuery = `DELETE FROM platform.traeger_master_data_audit
		WHERE organization_id = ? AND occurred_at < CURRENT_TIMESTAMP - INTERVAL '` + traegerAuditRetention + `'`
	appendTraegerMasterDataAuditQuery = `INSERT INTO platform.traeger_master_data_audit
		(organization_id, subject, subject_id, action, old_value, new_value, actor_account_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)`
)

// traegerMasterDataRow is one bank account of the Träger next to its
// profile; a Träger without accounts yields one row without them, and one
// without a profile row has no profile columns.
type traegerMasterDataRow struct {
	HasProfile bool `bun:"has_profile"`
	traegerProfileRow
	BankAccountID *int64                `bun:"bank_account_id"`
	BankAccount   traegerBankAccountRow `bun:"embed:bank_"`
}

// FindMasterData reads the profile and the bank accounts in one statement,
// so both come from one READ COMMITTED snapshot (#5571, decision E2: one
// early shared read may replace the former two reads; the read takes no
// lock).
func (s *TraegerMasterDataStore) FindMasterData(ctx context.Context, organizationID int64) (organizationtenancy.TraegerProfile, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return organizationtenancy.TraegerProfile{}, err
	}
	var rows []traegerMasterDataRow
	if err := db.NewRaw(findTraegerMasterDataQuery, organizationID).Scan(ctx, &rows); err != nil {
		return organizationtenancy.TraegerProfile{}, fmt.Errorf("organization tenancy postgres: find Träger master data: %w", err)
	}
	accounts := make([]organizationtenancy.TraegerBankAccount, 0, len(rows))
	profiles := make([]traegerProfileRow, 0, 1)
	for i, row := range rows {
		if i == 0 && row.HasProfile {
			profiles = append(profiles, row.traegerProfileRow)
		}
		if row.BankAccountID != nil {
			accounts = append(accounts, row.BankAccount.toDomain())
		}
	}
	profile := firstProfile(profiles)
	profile.BankAccounts = accounts
	return profile, nil
}

// FindDocumentContact reads the contact person and the withdrawal contact.
func (s *TraegerMasterDataStore) FindDocumentContact(ctx context.Context, organizationID int64) (organizationtenancy.TraegerDocumentContact, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return organizationtenancy.TraegerDocumentContact{}, err
	}
	var rows []traegerProfileRow
	if err := db.NewRaw(findTraegerProfileQuery, organizationID).Scan(ctx, &rows); err != nil {
		return organizationtenancy.TraegerDocumentContact{}, fmt.Errorf("organization tenancy postgres: find Träger document contact: %w", err)
	}
	profile := firstProfile(rows)
	return organizationtenancy.TraegerDocumentContact{Contact: profile.Contact, Withdrawal: profile.Withdrawal}, nil
}

// TraegerName serves organizationtenancy.TraegerNames through
// platform.current_traeger_name (1.15.898, #6437): the function answers only
// for the Träger of app.current_org_id, which traegerDB has checked to be
// organizationID.
func (s *TraegerMasterDataStore) TraegerName(ctx context.Context, organizationID int64) (string, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return "", err
	}
	var name *string
	if err := db.NewRaw(`SELECT platform.current_traeger_name()`).Scan(ctx, &name); err != nil {
		return "", fmt.Errorf("organization tenancy postgres: find Träger name: %w", err)
	}
	if name == nil {
		return "", nil
	}
	return *name, nil
}

// LockProfile locks the row; create makes it first where it is missing.
func (s *TraegerMasterDataStore) LockProfile(ctx context.Context, organizationID int64, create bool) (organizationtenancy.TraegerProfile, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return organizationtenancy.TraegerProfile{}, err
	}
	if create {
		if _, err := db.ExecContext(ctx, ensureTraegerProfileQuery, organizationID); err != nil {
			return organizationtenancy.TraegerProfile{}, fmt.Errorf("organization tenancy postgres: create Träger profile: %w", err)
		}
	}
	var rows []traegerProfileRow
	if err := db.NewRaw(lockTraegerProfileQuery, organizationID).Scan(ctx, &rows); err != nil {
		return organizationtenancy.TraegerProfile{}, fmt.Errorf("organization tenancy postgres: lock Träger profile: %w", err)
	}
	return firstProfile(rows), nil
}

func firstProfile(rows []traegerProfileRow) organizationtenancy.TraegerProfile {
	if len(rows) == 0 {
		return organizationtenancy.TraegerProfile{}
	}
	return rows[0].toDomain()
}

// SaveContact stores the contact person on the locked row.
func (s *TraegerMasterDataStore) SaveContact(ctx context.Context, organizationID, accountID int64, contact organizationtenancy.TraegerContact) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, saveTraegerContactQuery, contact.Name, contact.Phone, contact.Email, traegerActor(accountID), organizationID); err != nil {
		return fmt.Errorf("organization tenancy postgres: save Träger contact: %w", err)
	}
	return nil
}

// SaveWithdrawalContact stores the withdrawal contact on the locked row.
func (s *TraegerMasterDataStore) SaveWithdrawalContact(ctx context.Context, organizationID, accountID int64, contact organizationtenancy.TraegerWithdrawalContact) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, saveTraegerWithdrawalContactQuery, contact.Phone, contact.Email, traegerActor(accountID), organizationID); err != nil {
		return fmt.Errorf("organization tenancy postgres: save Träger withdrawal contact: %w", err)
	}
	return nil
}

// SaveLogo stores the logo on the locked row.
func (s *TraegerMasterDataStore) SaveLogo(ctx context.Context, organizationID, accountID int64, logo domain.TraegerLogoWrite) (organizationtenancy.TraegerLogo, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return organizationtenancy.TraegerLogo{}, err
	}
	var rows []traegerProfileRow
	if err := db.NewRaw(saveTraegerLogoQuery, logo.Data, logo.ContentType, logo.Width, logo.Height, traegerActor(accountID), organizationID).Scan(ctx, &rows); err != nil {
		return organizationtenancy.TraegerLogo{}, fmt.Errorf("organization tenancy postgres: save Träger logo: %w", err)
	}
	if len(rows) == 0 {
		return organizationtenancy.TraegerLogo{}, errors.New("organization tenancy postgres: save Träger logo: the profile row is missing")
	}
	saved := rows[0].toDomain().Logo
	if saved == nil {
		return organizationtenancy.TraegerLogo{}, errors.New("organization tenancy postgres: save Träger logo: no logo stored")
	}
	return *saved, nil
}

// ClearLogo deletes the logo from the locked row.
func (s *TraegerMasterDataStore) ClearLogo(ctx context.Context, organizationID, accountID int64) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, clearTraegerLogoQuery, traegerActor(accountID), organizationID); err != nil {
		return fmt.Errorf("organization tenancy postgres: clear Träger logo: %w", err)
	}
	return nil
}

type traegerLogoImageRow struct {
	ContentType string    `bun:"logo_content_type"`
	Data        []byte    `bun:"logo"`
	UpdatedAt   time.Time `bun:"logo_updated_at"`
}

// LogoImage reads the logo with its bytes.
func (s *TraegerMasterDataStore) LogoImage(ctx context.Context, organizationID int64) (organizationtenancy.TraegerLogoImage, bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return organizationtenancy.TraegerLogoImage{}, false, err
	}
	var rows []traegerLogoImageRow
	if err := db.NewRaw(traegerLogoImageQuery, organizationID).Scan(ctx, &rows); err != nil {
		return organizationtenancy.TraegerLogoImage{}, false, fmt.Errorf("organization tenancy postgres: read Träger logo: %w", err)
	}
	if len(rows) == 0 {
		return organizationtenancy.TraegerLogoImage{}, false, nil
	}
	return organizationtenancy.TraegerLogoImage(rows[0]), true, nil
}

// ListBankAccounts returns the bank accounts, the default first.
func (s *TraegerMasterDataStore) ListBankAccounts(ctx context.Context, organizationID int64) ([]organizationtenancy.TraegerBankAccount, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []traegerBankAccountRow
	if err := db.NewRaw(listTraegerBankAccountsQuery, organizationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("organization tenancy postgres: list Träger bank accounts: %w", err)
	}
	accounts := make([]organizationtenancy.TraegerBankAccount, 0, len(rows))
	for _, row := range rows {
		accounts = append(accounts, row.toDomain())
	}
	return accounts, nil
}

// InsertBankAccount adds a bank account.
func (s *TraegerMasterDataStore) InsertBankAccount(ctx context.Context, organizationID, accountID int64, input organizationtenancy.TraegerBankAccountInput) (organizationtenancy.TraegerBankAccount, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return organizationtenancy.TraegerBankAccount{}, err
	}
	var rows []traegerBankAccountRow
	err = db.NewRaw(insertTraegerBankAccountQuery, organizationID, input.Label, input.AccountHolder, input.IBAN, input.BIC,
		input.BankName, input.IsDefault, traegerActor(accountID), traegerActor(accountID)).Scan(ctx, &rows)
	return bankAccountResult(rows, err, "insert")
}

// UpdateBankAccount changes a bank account.
func (s *TraegerMasterDataStore) UpdateBankAccount(ctx context.Context, organizationID, accountID, bankAccountID int64, input organizationtenancy.TraegerBankAccountInput) (organizationtenancy.TraegerBankAccount, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return organizationtenancy.TraegerBankAccount{}, err
	}
	var rows []traegerBankAccountRow
	err = db.NewRaw(updateTraegerBankAccountQuery, input.Label, input.AccountHolder, input.IBAN, input.BIC, input.BankName,
		input.IsDefault, traegerActor(accountID), organizationID, bankAccountID).Scan(ctx, &rows)
	return bankAccountResult(rows, err, "update")
}

// bankAccountResult names a missing row. The application refuses a taken
// label before it writes; a unique violation here is a failure that rolls
// the Träger transaction back.
func bankAccountResult(rows []traegerBankAccountRow, err error, operation string) (organizationtenancy.TraegerBankAccount, error) {
	if err != nil {
		return organizationtenancy.TraegerBankAccount{}, fmt.Errorf("organization tenancy postgres: %s Träger bank account: %w", operation, err)
	}
	if len(rows) == 0 {
		return organizationtenancy.TraegerBankAccount{}, organizationtenancy.ErrTraegerBankAccountNotFound
	}
	return rows[0].toDomain(), nil
}

// ClearDefaultBankAccount takes the default flag from every bank account.
func (s *TraegerMasterDataStore) ClearDefaultBankAccount(ctx context.Context, organizationID int64) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, clearDefaultTraegerBankAccountQuery, organizationID); err != nil {
		return fmt.Errorf("organization tenancy postgres: clear default Träger bank account: %w", err)
	}
	return nil
}

// DeleteBankAccount deletes a bank account.
func (s *TraegerMasterDataStore) DeleteBankAccount(ctx context.Context, organizationID, bankAccountID int64) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, deleteTraegerBankAccountQuery, organizationID, bankAccountID); err != nil {
		return fmt.Errorf("organization tenancy postgres: delete Träger bank account: %w", err)
	}
	return nil
}

// AppendAudit prunes the Träger's expired entries and records one entry.
func (s *TraegerMasterDataStore) AppendAudit(ctx context.Context, entry domain.TraegerMasterDataAuditEntry) error {
	db, err := traegerDB(ctx, entry.OrganizationID)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, pruneTraegerMasterDataAuditQuery, entry.OrganizationID); err != nil {
		return fmt.Errorf("organization tenancy postgres: prune Träger master data log: %w", err)
	}
	var subjectID *int64
	if entry.SubjectID > 0 {
		subjectID = &entry.SubjectID
	}
	if _, err := db.ExecContext(ctx, appendTraegerMasterDataAuditQuery, entry.OrganizationID, entry.Subject, subjectID, entry.Action,
		jsonOrNull(entry.OldValue), jsonOrNull(entry.NewValue), traegerActor(entry.ActorAccountID)); err != nil {
		return fmt.Errorf("organization tenancy postgres: append Träger master data log: %w", err)
	}
	return nil
}

// traegerActor binds the acting account, or NULL for none.
func traegerActor(accountID int64) *int64 {
	if accountID <= 0 {
		return nil
	}
	return &accountID
}

// jsonOrNull binds a JSON value, or NULL for none.
func jsonOrNull(value []byte) *string {
	if len(value) == 0 {
		return nil
	}
	text := string(value)
	return &text
}
