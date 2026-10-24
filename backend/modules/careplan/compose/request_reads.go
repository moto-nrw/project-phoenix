package compose

import (
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	careRecordPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
)

func NewRequestReads(records careRecordPorts.RequestReadRecords, diffs carerequests.Diffs, logger *slog.Logger) (carerequests.Reads, error) {
	if records == nil || diffs == nil {
		return nil, errors.New("care request reads: records and diffs are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &application.RequestReads{Records: application.RequestReadRecordsAdapter{RequestReadRecords: records}, Diffs: diffs, Logger: logger}, nil
}

// NewParentCareRequestQueries retains request lookup/decoding for parent sharing.
func NewParentCareRequestQueries(records application.ParentCareRequestRecords, missing func() error) carerequests.ParentCareRequestQueries {
	return application.ParentCareRequestReads{Records: records, Missing: missing}
}
