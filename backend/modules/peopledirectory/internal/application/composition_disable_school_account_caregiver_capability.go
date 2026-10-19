package application

import (
	"context"
	"encoding/json"
)

func DisableSchoolAccountCaregiverCapability(ctx context.Context, schoolID, accountID int64, inSchool func(context.Context, int64, func(context.Context) (json.RawMessage, error)) (json.RawMessage, error), command func(context.Context, int64) (json.RawMessage, error)) (json.RawMessage, error) {
	return inSchool(ctx, schoolID, func(ctx context.Context) (json.RawMessage, error) {
		return command(ctx, accountID)
	})
}
