package enrollment_test

import (
	"context"
	"errors"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

func (failingSchemaRepo) ListByIDs(context.Context, []int64) ([]*enrollmentModels.FormSchema, error) {
	return nil, errors.New("schema read failed")
}

func (failingPhaseRepo) ListByIDs(context.Context, []int64) ([]*enrollmentModels.Phase, error) {
	return nil, errors.New("phase read failed")
}
