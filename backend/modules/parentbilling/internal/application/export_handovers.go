package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// The handover of a finance export to the recipient of the Träger (#6824,
// ADR 0218). A handover is an export like a download: the same producer
// makes the file, takes the next file number and writes the protocol entry.
// Only the target differs. Export Transfer runs the producer in the Träger
// transaction of its transfer intent and the settlement in the one that
// completes its journal, so protocol and journal always agree.

// ExportHandoverStore records the handovers of protocol entries. Every
// method runs in the Träger transaction of organizationID.
type ExportHandoverStore interface {
	// SameContentHandedOver tells whether another file of the same run and
	// format with the same content was handed over, is under way or has an
	// unknown outcome.
	SameContentHandedOver(ctx context.Context, organizationID, exportFileID int64) (bool, error)
	InsertExportFileHandover(ctx context.Context, organizationID, exportFileID int64, host, directory string) error
	SettleExportFileHandover(ctx context.Context, organizationID, exportFileID int64, state, reason string) error
}

// ExportHandovers serves parentbilling.FinanceExportHandovers.
type ExportHandovers struct {
	exports  parentbilling.FinanceExports
	debits   parentbilling.SEPADebits
	store    ExportHandoverStore
	transfer parentbilling.FinanceExportTransfer
}

var _ parentbilling.FinanceExportHandovers = (*ExportHandovers)(nil)

// NewExportHandovers builds the handovers over the producers of the
// downloads and the transport the root binds.
func NewExportHandovers(exports parentbilling.FinanceExports, debits parentbilling.SEPADebits, store ExportHandoverStore,
	transfer parentbilling.FinanceExportTransfer,
) *ExportHandovers {
	if exports == nil || debits == nil || store == nil || transfer == nil {
		panic("parent billing: the exports, the debits, the store and the transfer are required for handovers")
	}
	return &ExportHandovers{exports: exports, debits: debits, store: store, transfer: transfer}
}

// producedFile is a file a producer made and recorded.
type producedFile struct {
	exportFileID int64
	fileName     string
	data         []byte
	// host and directory are where the recipient puts it.
	host, directory string
}

// producer makes the file of the request in the given Träger transaction.
func (h *ExportHandovers) producer(reviewer parentbilling.HolidayReviewer, request parentbilling.HandoverRequest,
) (func(context.Context) (producedFile, error), error) {
	switch request.Format {
	case parentbilling.ExportFormatSollstellung:
		return func(ctx context.Context) (producedFile, error) {
			file, err := h.exports.SollstellungFile(ctx, reviewer, request.RunKind, request.RunID)
			return producedFile{exportFileID: file.Entry.ID, fileName: file.Entry.FileName, data: file.Data}, err
		}, nil
	case parentbilling.ExportFormatDatev:
		return func(ctx context.Context) (producedFile, error) {
			file, err := h.exports.DatevFile(ctx, reviewer, request.RunKind, request.RunID, request.Datev)
			return producedFile{exportFileID: file.Entry.ID, fileName: file.Entry.FileName, data: file.Data}, err
		}, nil
	case parentbilling.ExportFormatPain008:
		return func(ctx context.Context) (producedFile, error) {
			file, err := h.debits.SEPADebitFile(ctx, reviewer, request.RunKind, request.RunID)
			return producedFile{exportFileID: file.ExportFileID, fileName: file.Filename, data: file.Data}, err
		}, nil
	}
	return nil, parentbilling.ErrExportHandoverFormat
}

// HandOver makes the file of a run and hands it to the recipient. A content
// that already arrived, is under way or has an unknown outcome is refused
// before its number is taken, unless the request repeats it on purpose.
func (h *ExportHandovers) HandOver(ctx context.Context, reviewer parentbilling.HolidayReviewer, request parentbilling.HandoverRequest,
) (parentbilling.HandoverResult, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.HandoverResult{}, err
	}
	produce, err := h.producer(reviewer, request)
	if err != nil {
		return parentbilling.HandoverResult{}, err
	}
	var made producedFile
	outcome, err := h.transfer.Handover(ctx, parentbilling.FinanceHandoverRequest{
		OrganizationID: reviewer.OrganizationID, AccountID: reviewer.AccountID, Format: request.Format,
		Prepare: func(txCtx context.Context, host, directory string) (string, []byte, error) {
			file, err := h.prepare(txCtx, reviewer.OrganizationID, request.Repeat, produce, host, directory)
			made = file
			return file.fileName, file.data, err
		},
		Settle: func(txCtx context.Context, transferred bool, reason string) error {
			state := parentbilling.HandoverFailed
			if transferred {
				state, reason = parentbilling.HandoverTransferred, ""
			}
			return h.store.SettleExportFileHandover(txCtx, reviewer.OrganizationID, made.exportFileID, state, reason)
		},
	})
	if err != nil {
		return parentbilling.HandoverResult{}, err
	}
	if outcome.NotReady {
		return parentbilling.HandoverResult{}, parentbilling.ErrExportRecipientNotReady
	}
	return handoverResult(made, outcome), nil
}

// prepare makes the file, refuses a repeated content without Repeat and
// records the pending handover, all in the transfer intent's transaction.
func (h *ExportHandovers) prepare(ctx context.Context, organizationID int64, repeat bool,
	produce func(context.Context) (producedFile, error), host, directory string,
) (producedFile, error) {
	file, err := produce(ctx)
	if err != nil {
		return producedFile{}, err
	}
	if file.exportFileID <= 0 {
		return producedFile{}, errors.New("parent billing: the handed over file has no protocol entry")
	}
	if !repeat {
		repeated, err := h.store.SameContentHandedOver(ctx, organizationID, file.exportFileID)
		if err != nil {
			return producedFile{}, fmt.Errorf("parent billing: check earlier handovers: %w", err)
		}
		if repeated {
			// The rollback of the intent also returns the file number.
			return producedFile{}, parentbilling.ErrExportAlreadyHandedOver
		}
	}
	if err := h.store.InsertExportFileHandover(ctx, organizationID, file.exportFileID, host, directory); err != nil {
		return producedFile{}, fmt.Errorf("parent billing: record the handover: %w", err)
	}
	file.host, file.directory = host, directory
	return file, nil
}

func handoverResult(made producedFile, outcome parentbilling.FinanceHandoverOutcome) parentbilling.HandoverResult {
	result := parentbilling.HandoverResult{FileName: made.fileName, Reason: outcome.Reason,
		TargetHost: made.host, TargetDirectory: made.directory}
	switch {
	case outcome.Uncertain:
		result.State, result.Reason = parentbilling.HandoverUnknown, ""
	case outcome.Transferred:
		result.State, result.Reason = parentbilling.HandoverTransferred, ""
	default:
		result.State = parentbilling.HandoverFailed
	}
	return result
}
