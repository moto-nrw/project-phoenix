package users

import (
	"context"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	models "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

type nativeGuardianPaymentRepository struct {
	raw *GuardianFinancialDataRepository
}

func NewGuardianFinancialDataRepository(runtime Runtime) ports.GuardianFinancialDataRepository {
	return nativeGuardianPaymentRepository{raw: newRawGuardianFinancialDataRepository(runtime)}
}
func nativeGuardianPayment(value *models.GuardianFinancialData) *directory.GuardianPayment {
	if value == nil {
		return nil
	}
	return &directory.GuardianPayment{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, GuardianProfileID: value.GuardianProfileID, IBAN: value.IBAN, AccountHolder: value.AccountHolder}
}
func guardianPaymentRow(value *directory.GuardianPayment) *models.GuardianFinancialData {
	if value == nil {
		return nil
	}
	row := &models.GuardianFinancialData{GuardianProfileID: value.GuardianProfileID, IBAN: value.IBAN, AccountHolder: value.AccountHolder}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row
}
func (r nativeGuardianPaymentRepository) Create(ctx context.Context, value *directory.GuardianPayment) error {
	row := guardianPaymentRow(value)
	err := r.raw.Create(ctx, row)
	if value != nil && row != nil {
		*value = *nativeGuardianPayment(row)
	}
	return err
}
func (r nativeGuardianPaymentRepository) Update(ctx context.Context, value *directory.GuardianPayment) error {
	row := guardianPaymentRow(value)
	err := r.raw.Update(ctx, row)
	if value != nil && row != nil {
		*value = *nativeGuardianPayment(row)
	}
	return err
}
func (r nativeGuardianPaymentRepository) FindByGuardianProfileID(ctx context.Context, id int64) (*directory.GuardianPayment, error) {
	row, err := r.raw.FindByGuardianProfileID(ctx, id)
	return nativeGuardianPayment(row), err
}
func (r nativeGuardianPaymentRepository) ListByGuardianProfileIDs(ctx context.Context, ids []int64) (map[int64]*directory.GuardianPayment, error) {
	rows, err := r.raw.ListByGuardianProfileIDs(ctx, ids)
	return mapRecordValues(rows, nativeGuardianPayment), err
}
