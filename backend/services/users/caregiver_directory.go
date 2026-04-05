package users

import (
	"context"
	"database/sql"
	"fmt"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// CaregiverDirectory exposes the canonical operational caregiver lookup.
// It is intentionally separate from PersonService so existing mocks do not
// need to implement these methods unless a test actually exercises them.
type CaregiverDirectory interface {
	ListActiveCaregivers(ctx context.Context) ([]*userModels.ActiveCaregiver, error)
	FindActiveCaregiverByAccountID(ctx context.Context, accountID int64) (*userModels.ActiveCaregiver, error)
}

func (s *personService) ListActiveCaregivers(ctx context.Context) ([]*userModels.ActiveCaregiver, error) {
	var caregivers []*userModels.ActiveCaregiver
	query := s.caregiverDirectoryQuery(ctx).
		OrderExpr(`"person".first_name ASC, "person".last_name ASC, "staff".id ASC`)

	if err := query.Scan(ctx, &caregivers); err != nil {
		return nil, &UsersError{Op: "list active caregivers", Err: err}
	}

	return caregivers, nil
}

func (s *personService) FindActiveCaregiverByAccountID(ctx context.Context, accountID int64) (*userModels.ActiveCaregiver, error) {
	var caregiver userModels.ActiveCaregiver
	query := s.caregiverDirectoryQuery(ctx).
		Where(`"account".id = ?`, accountID).
		Limit(1)

	if err := query.Scan(ctx, &caregiver); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, &UsersError{Op: "find active caregiver by account ID", Err: err}
	}

	return &caregiver, nil
}

func (s *personService) caregiverDirectoryQuery(ctx context.Context) *bun.SelectQuery {
	db := bun.IDB(s.db)
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		db = tx
	}

	query := db.NewSelect().
		Model((*userModels.ActiveCaregiver)(nil)).
		ModelTableExpr(`users.teachers AS "teacher"`).
		ColumnExpr(`"account".id AS account_id`).
		ColumnExpr(`"person".id AS person_id`).
		ColumnExpr(`"staff".id AS staff_id`).
		ColumnExpr(`"teacher".id AS teacher_id`).
		ColumnExpr(`"person".first_name`).
		ColumnExpr(`"person".last_name`).
		ColumnExpr(`"account".email`).
		ColumnExpr(`"staff".created_at`).
		ColumnExpr(`"staff".updated_at`).
		Join(`INNER JOIN users.staff AS "staff" ON "staff".id = "teacher".staff_id`).
		Join(`INNER JOIN users.persons AS "person" ON "person".id = "staff".person_id`).
		Join(`INNER JOIN auth.accounts AS "account" ON "account".id = "person".account_id`).
		Join(`INNER JOIN auth.account_roles AS "ar" ON "ar".account_id = "account".id`).
		Join(`INNER JOIN auth.roles AS "role" ON "role".id = "ar".role_id`).
		Where(`"account".active = TRUE`).
		Where(`"role".is_system = TRUE`).
		Where(`"role".tenant_id IS NULL`).
		Where(`LOWER("role".name) = ?`, "user").
		Distinct()

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.
			Where(`"teacher".tenant_id = ?`, tenantID).
			Where(`"staff".tenant_id = ?`, tenantID).
			Where(`"person".tenant_id = ?`, tenantID).
			Where(`"ar".tenant_id = ?`, tenantID)
	}

	return query
}

func CaregiverDirectoryFromPersonService(personService PersonService) (CaregiverDirectory, error) {
	directory, ok := personService.(CaregiverDirectory)
	if !ok {
		return nil, fmt.Errorf("person service does not implement caregiver directory")
	}
	return directory, nil
}
