package api

import (
	"errors"
	"fmt"

	iotAPI "github.com/moto-nrw/project-phoenix/api/iot"
	"github.com/moto-nrw/project-phoenix/observability"
)

// NewIoTErrorReportRelay binds parsing/tagging; the HTTP adapter retains all network I/O.
func NewIoTErrorReportRelay(dsn string) (iotAPI.ErrorReportRelay, error) {
	return iotAPI.NewErrorReportRelay(dsn, iotAPI.SentryTransportRuntime{
		ParseTarget: func(dsn string) (iotAPI.SentryTarget, error) {
			target, err := observability.ParseSentryDSN(dsn)
			return iotAPI.SentryTarget{ProjectID: target.ProjectID, EnvelopeURL: target.EnvelopeURL}, err
		},
		TagEnvelope: func(envelope []byte, projectID string, origin iotAPI.ErrorReportOrigin) ([]byte, error) {
			tagged, err := observability.TagSentryEnvelope(envelope, projectID, observability.SentryReporter{DeviceID: origin.DeviceID, SchoolID: origin.SchoolID})
			switch {
			case errors.Is(err, observability.ErrSentryProjectNotAllowed):
				return nil, fmt.Errorf("%w: %w", iotAPI.ErrErrorReportProjectNotAllowed, err)
			case err != nil:
				return nil, fmt.Errorf("%w: %w", iotAPI.ErrErrorReportInvalid, err)
			}
			return tagged, nil
		},
	})
}
