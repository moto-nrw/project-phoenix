//go:build testhooks

package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
)

func mirrorSchoolID(ctx context.Context) int64 { return application.MirrorSchoolID(ctx) }
