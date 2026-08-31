package platform

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/uptrace/bun"
)

// platformWhereID is the shared "id = ?" predicate used by the operator MFA
// repos. Defined here so all four files can reuse a single const.
const platformWhereID = "id = ?"

const (
	operatorMFACredentialTable      = "platform.operator_mfa_credentials"
	operatorMFACredentialTableAlias = `platform.operator_mfa_credentials AS "operator_mfa_credential"`
)

// OperatorMFACredentialRepository implements platform.OperatorMFACredentialRepository.
type OperatorMFACredentialRepository struct {
	*base.Repository[*platform.OperatorMFACredential]
	db *bun.DB
}

// NewOperatorMFACredentialRepository creates a new repository for operator
// MFA credential records.
func NewOperatorMFACredentialRepository(db *bun.DB) platform.OperatorMFACredentialRepository {
	return &OperatorMFACredentialRepository{
		Repository: base.NewRepository[*platform.OperatorMFACredential](db, operatorMFACredentialTable, "OperatorMfaCredential"),
		db:         db,
	}
}

func (r *OperatorMFACredentialRepository) Update(ctx context.Context, credential *platform.OperatorMFACredential) error {
	if credential == nil {
		return fmt.Errorf("operator mfa credential cannot be nil")
	}
	if err := credential.Validate(); err != nil {
		return err
	}
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model(credential).
		ModelTableExpr(operatorMFACredentialTable).
		Where(platformWhereID, credential.ID).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "update operator mfa credential", Err: base.TranslateNotFound(err)}
	}
	return nil
}

func (r *OperatorMFACredentialRepository) FindByOperatorID(ctx context.Context, operatorID int64) (*platform.OperatorMFACredential, error) {
	credential := new(platform.OperatorMFACredential)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(credential).
		ModelTableExpr(operatorMFACredentialTableAlias).
		Where("operator_id = ?", operatorID).
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find operator mfa credential by operator id", Err: base.TranslateNotFound(err)}
	}
	return credential, nil
}

func (r *OperatorMFACredentialRepository) UpdateLastUsedAt(ctx context.Context, id int64, when time.Time) error {
	credential := &platform.OperatorMFACredential{Model: modelBase.Model{ID: id}, LastUsedAt: &when}
	_, err := r.UpdateColumns(ctx, credential, "last_used_at")
	return err
}

func (r *OperatorMFACredentialRepository) DeleteByOperatorID(ctx context.Context, operatorID int64) error {
	_, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*platform.OperatorMFACredential)(nil)).
		ModelTableExpr(operatorMFACredentialTable).
		Where("operator_id = ?", operatorID).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "delete operator mfa credential by operator id", Err: base.TranslateNotFound(err)}
	}
	return nil
}

func (r *OperatorMFACredentialRepository) List(ctx context.Context, filters map[string]interface{}) ([]*platform.OperatorMFACredential, error) {
	var credentials []*platform.OperatorMFACredential
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&credentials).
		ModelTableExpr(operatorMFACredentialTableAlias)
	for field, value := range filters {
		if value != nil {
			query = query.Where("? = ?", bun.Ident(field), value)
		}
	}
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list operator mfa credentials", Err: base.TranslateNotFound(err)}
	}
	return credentials, nil
}
