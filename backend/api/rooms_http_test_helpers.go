//go:build testhooks

package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
)

// RoomsHTTPBindingsForTest supplies the same root-owned callbacks as the room mount.
func RoomsHTTPBindingsForTest() (func(context.Context, any, ...string) context.Context, func(error) bool) {
	return settings.PrefetchSettings, transactionruntime.IsConstraintViolation
}
