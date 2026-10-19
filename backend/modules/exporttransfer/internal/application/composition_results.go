package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/domain"
)

func StatusBinding[R any](ctx context.Context, service *Service, construct func(bool, bool, string, int, string, []string) R) (R, error) {
	state, err := service.State(ctx)
	if err != nil {
		var zero R
		return zero, err
	}
	return construct(state.Enabled, state.Ready(), state.Host, state.Port, state.RemoteDirectory, state.MissingSettings), nil
}
func TransferBinding[R any](ctx context.Context, service *Service, request domain.Request, construct func(bool, string, int64, string, string, string) R) (R, error) {
	result, err := service.Transfer(ctx, request)
	if err != nil {
		var zero R
		return zero, err
	}
	return construct(result.Success, result.Filename, result.ByteSize, result.TargetHost, result.TargetDirectory, result.Reason), nil
}
