package enrollment_test

import (
	"context"
	"errors"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func (failingSchemaRepo) Schemas(context.Context, []int64) ([]*capability.FormSchema, error) {
	return nil, errors.New("schema read failed")
}

func (failingPhaseRepo) PhasesByID(context.Context, []int64) ([]*capability.Phase, error) {
	return nil, errors.New("phase read failed")
}
