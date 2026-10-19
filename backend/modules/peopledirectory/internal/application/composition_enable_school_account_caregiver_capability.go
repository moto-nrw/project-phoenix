package application

import (
	"context"
	"encoding/json"
)

func EnableSchoolAccountCaregiverCapability(ctx context.Context, schoolID, accountID int64, firstName, lastName, position string, inSchool func(context.Context, int64, func(context.Context) (json.RawMessage, error)) (json.RawMessage, error), command func(context.Context, int64, string, string, string) (json.RawMessage, error)) (json.RawMessage, error) {
	return inSchool(ctx, schoolID, func(ctx context.Context) (json.RawMessage, error) {
		return command(ctx, accountID, firstName, lastName, position)
	})
}
