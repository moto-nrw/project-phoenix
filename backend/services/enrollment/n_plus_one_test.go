package enrollment_test

import (
	"context"
	"errors"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

func (failingSchemaRepo) ListWithOptions(context.Context, *modelBase.QueryOptions) ([]*enrollmentModels.FormSchema, error) {
	return nil, errors.New("schema read failed")
}

func (failingPhaseRepo) ListByIDs(context.Context, []int64) ([]*enrollmentModels.Phase, error) {
	return nil, errors.New("phase read failed")
}
