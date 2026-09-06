package enrollment

import (
	"context"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// transactionalFormSchemaService keeps each lineage mutation and its phase
// repointing in the caller transaction, or opens one for standalone callers.
type transactionalFormSchemaService struct{ FormSchemaService }

func (s *transactionalFormSchemaService) CreateSchema(ctx context.Context, name string, fields []enrollmentModels.FormField, createdBy int64, coreRequirements ...enrollmentModels.CoreRequirements) (*capability.FormSchema, error) {
	var result *capability.FormSchema
	err := tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		var operationErr error
		result, operationErr = s.FormSchemaService.CreateSchema(txCtx, name, fields, createdBy, coreRequirements...)
		return operationErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *transactionalFormSchemaService) CreateSchemaWithLegal(ctx context.Context, name string, fields []enrollmentModels.FormField, createdBy int64, coreRequirements enrollmentModels.CoreRequirements, legalBlocks []enrollmentModels.FormLegalBlock) (*capability.FormSchema, error) {
	var result *capability.FormSchema
	err := tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		var operationErr error
		result, operationErr = s.FormSchemaService.CreateSchemaWithLegal(txCtx, name, fields, createdBy, coreRequirements, legalBlocks)
		return operationErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *transactionalFormSchemaService) UpdateSchema(ctx context.Context, id int64, fields []enrollmentModels.FormField, updatedBy int64, coreRequirements ...enrollmentModels.CoreRequirements) (*capability.FormSchema, error) {
	var result *capability.FormSchema
	err := tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		var operationErr error
		result, operationErr = s.FormSchemaService.UpdateSchema(txCtx, id, fields, updatedBy, coreRequirements...)
		return operationErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *transactionalFormSchemaService) UpdateSchemaWithLegal(ctx context.Context, id int64, fields []enrollmentModels.FormField, updatedBy int64, coreRequirements *enrollmentModels.CoreRequirements, legalBlocks *[]enrollmentModels.FormLegalBlock) (*capability.FormSchema, error) {
	var result *capability.FormSchema
	err := tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		var operationErr error
		result, operationErr = s.FormSchemaService.UpdateSchemaWithLegal(txCtx, id, fields, updatedBy, coreRequirements, legalBlocks)
		return operationErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *transactionalFormSchemaService) RenameSchema(ctx context.Context, id int64, newName string) (*capability.FormSchema, error) {
	var result *capability.FormSchema
	err := tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		var operationErr error
		result, operationErr = s.FormSchemaService.RenameSchema(txCtx, id, newName)
		return operationErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *transactionalFormSchemaService) DeleteSchema(ctx context.Context, id int64) error {
	return tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error { return s.FormSchemaService.DeleteSchema(txCtx, id) })
}

func (s *transactionalFormSchemaService) PublishForm(ctx context.Context, in PublishFormInput) (*capability.FormSchema, error) {
	var result *capability.FormSchema
	err := tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		var operationErr error
		result, operationErr = s.FormSchemaService.PublishForm(txCtx, in)
		return operationErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *transactionalFormSchemaService) PublishFormVersion(ctx context.Context, in PublishFormVersionInput) (*capability.FormSchema, error) {
	var result *capability.FormSchema
	err := tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		var operationErr error
		result, operationErr = s.FormSchemaService.PublishFormVersion(txCtx, in)
		return operationErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
