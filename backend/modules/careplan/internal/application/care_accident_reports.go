package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Accident reports (Unfallanzeige, #4771) are child documents of the
// category unfallanzeige that carry what was recorded. They follow the
// document rules: the per-child gate and every change-history row run in the
// command's own tenant transaction, and the PDF download is refused when its
// data-access log row cannot be written. Reading needs student_accidents:read,
// every write student_accidents:manage on top.

var _ careplan.StudentAccidentReports = (*StudentDocuments)(nil)

// accidentReportContentType is the type of the rendered report.
const accidentReportContentType = "application/pdf"

func (s *StudentDocuments) requireAccidentManage(actor careplan.StudentDocumentActor) error {
	if actor.AccountID <= 0 {
		return errors.New("actor account id is required")
	}
	return s.requireWritePermission(careplan.StudentDocumentCategoryUnfallanzeige, actor)
}

func (s *StudentDocuments) CreateStudentAccidentReport(ctx context.Context, studentID int64, report careplan.AccidentReport, actor careplan.StudentDocumentActor) (careplan.CareDocument, error) {
	if err := s.requireAccidentManage(actor); err != nil {
		return careplan.CareDocument{}, err
	}
	report, err := careplan.NormalizeAccidentReport(report)
	if err != nil {
		return careplan.CareDocument{}, err
	}
	var created careplan.CareDocument
	err = s.transaction(ctx, func(txCtx context.Context) error {
		if err := s.requireStudentAccess(txCtx, studentID, actor); err != nil {
			return err
		}
		created, err = s.createAccidentReport(txCtx, studentID, report, nil, actor)
		if err != nil {
			return err
		}
		return s.recordChange(txCtx, studentID, actor, created.Category, "", accidentAuditValue(created, "erfasst"))
	})
	if err != nil {
		return careplan.CareDocument{}, err
	}
	return created, nil
}

func (s *StudentDocuments) createAccidentReport(ctx context.Context, studentID int64, report careplan.AccidentReport, supersedes *int64, actor careplan.StudentDocumentActor) (careplan.CareDocument, error) {
	created, err := s.records.CreateCareDocument(ctx, careplan.CareDocument{
		StudentID: studentID, Category: careplan.StudentDocumentCategoryUnfallanzeige,
		FilenameDisplay: accidentReportFilename(report), ContentType: accidentReportContentType,
		UploadedBy: actor.AccountID, AccidentReport: &report, AccidentSupersedesID: supersedes,
	})
	if err != nil {
		return careplan.CareDocument{}, fmt.Errorf("care plan documents: create accident report: %w", err)
	}
	return created, nil
}

func (s *StudentDocuments) UpdateStudentAccidentReport(ctx context.Context, studentID, documentID int64, report careplan.AccidentReport, actor careplan.StudentDocumentActor) (careplan.CareDocument, error) {
	if err := s.requireAccidentManage(actor); err != nil {
		return careplan.CareDocument{}, err
	}
	report, err := careplan.NormalizeAccidentReport(report)
	if err != nil {
		return careplan.CareDocument{}, err
	}
	var updated careplan.CareDocument
	err = s.transaction(ctx, func(txCtx context.Context) error {
		document, err := s.findOpenAccidentReport(txCtx, studentID, documentID, actor)
		if err != nil {
			return err
		}
		before := accidentAuditValue(document, "")
		document.AccidentReport = &report
		document.FilenameDisplay = accidentReportFilename(report)
		if updated, err = s.records.UpdateOpenCareAccidentReport(txCtx, document); err != nil {
			return err
		}
		return s.recordChange(txCtx, studentID, actor, updated.Category, before, accidentAuditValue(updated, "geändert"))
	})
	if err != nil {
		return careplan.CareDocument{}, err
	}
	return updated, nil
}

func (s *StudentDocuments) SignStudentAccidentReport(ctx context.Context, studentID, documentID int64, signature careplan.AccidentSignatureInput, actor careplan.StudentDocumentActor) (careplan.CareDocument, error) {
	if err := s.requireAccidentManage(actor); err != nil {
		return careplan.CareDocument{}, err
	}
	signer := strings.TrimSpace(actor.Name)
	if signer == "" {
		return careplan.CareDocument{}, fmt.Errorf("%w: the signer needs a name", careplan.ErrAccidentSignatureInvalid)
	}
	var signed careplan.CareDocument
	err := s.transaction(ctx, func(txCtx context.Context) error {
		document, err := s.findOpenAccidentReport(txCtx, studentID, documentID, actor)
		if err != nil {
			return err
		}
		statement, err := careplan.AccidentReportStatement(document)
		if err != nil {
			return err
		}
		now := s.now()
		signatureID, err := s.signatures.RecordAccidentSignature(txCtx, careplan.AccidentSignatureEntry{
			StudentID: studentID, SignerAccountID: actor.AccountID, SignerName: signer,
			Statement: statement, SignedAt: now, Input: signature,
		})
		if err != nil {
			return err
		}
		signedBy := actor.AccountID
		document.AccidentSignatureID, document.AccidentSignedAt, document.AccidentSignedBy = &signatureID, &now, &signedBy
		if signed, err = s.records.UpdateOpenCareAccidentReport(txCtx, document); err != nil {
			return err
		}
		return s.recordChange(txCtx, studentID, actor, signed.Category, accidentAuditValue(document, ""), accidentAuditValue(signed, "unterschrieben von "+signer))
	})
	if err != nil {
		return careplan.CareDocument{}, err
	}
	return signed, nil
}

func (s *StudentDocuments) CorrectStudentAccidentReport(ctx context.Context, studentID, documentID int64, report careplan.AccidentReport, actor careplan.StudentDocumentActor) (careplan.CareDocument, error) {
	if err := s.requireAccidentManage(actor); err != nil {
		return careplan.CareDocument{}, err
	}
	report, err := careplan.NormalizeAccidentReport(report)
	if err != nil {
		return careplan.CareDocument{}, err
	}
	var corrected careplan.CareDocument
	err = s.transaction(ctx, func(txCtx context.Context) error {
		previous, err := s.findAccidentReport(txCtx, studentID, documentID, actor)
		if err != nil {
			return err
		}
		if previous.AccidentSignedAt == nil {
			return careplan.ErrAccidentReportNotSigned
		}
		previousID := previous.ID
		if corrected, err = s.createAccidentReport(txCtx, studentID, report, &previousID, actor); err != nil {
			return err
		}
		return s.recordChange(txCtx, studentID, actor, corrected.Category, accidentAuditValue(previous, ""), accidentAuditValue(corrected, "als neue Fassung korrigiert"))
	})
	if err != nil {
		return careplan.CareDocument{}, err
	}
	return corrected, nil
}

func (s *StudentDocuments) AuthorizeAccidentPaperCopy(ctx context.Context, studentID, documentID int64, actor careplan.StudentDocumentActor) error {
	if err := s.requireAccidentManage(actor); err != nil {
		return err
	}
	return s.transaction(ctx, func(txCtx context.Context) error {
		_, err := s.findOpenAccidentReport(txCtx, studentID, documentID, actor)
		return err
	})
}

func (s *StudentDocuments) AttachAccidentPaperCopy(ctx context.Context, input careplan.AccidentPaperCopyInput, actor careplan.StudentDocumentActor) (careplan.CareDocument, error) {
	if err := s.requireAccidentManage(actor); err != nil {
		return careplan.CareDocument{}, err
	}
	filename := strings.TrimSpace(input.FilenameDisplay)
	if filename == "" || strings.TrimSpace(input.FilenameStored) == "" || strings.TrimSpace(input.ContentType) == "" || input.SizeBytes < 0 {
		return careplan.CareDocument{}, fmt.Errorf("%w: the paper copy needs a stored file", careplan.ErrStudentDocumentInvalid)
	}
	var signed careplan.CareDocument
	err := s.transaction(ctx, func(txCtx context.Context) error {
		document, err := s.findOpenAccidentReport(txCtx, input.StudentID, input.DocumentID, actor)
		if err != nil {
			return err
		}
		before := accidentAuditValue(document, "")
		now, signedBy := s.now(), actor.AccountID
		document.FilenameDisplay, document.FilenameStored = filename, input.FilenameStored
		document.SizeBytes, document.ContentType = input.SizeBytes, input.ContentType
		document.AccidentSignedAt, document.AccidentSignedBy = &now, &signedBy
		if signed, err = s.records.UpdateOpenCareAccidentReport(txCtx, document); err != nil {
			return err
		}
		if err := s.records.CompleteCareDocumentCleanupByFilename(txCtx, input.FilenameStored); err != nil {
			return fmt.Errorf("complete accident paper copy cleanup intent: %w", err)
		}
		return s.recordChange(txCtx, input.StudentID, actor, signed.Category, before, accidentAuditValue(signed, "unterschriebenes Papierexemplar hochgeladen"))
	})
	if err != nil {
		return careplan.CareDocument{}, err
	}
	return signed, nil
}

func (s *StudentDocuments) ResolveStudentAccidentReportFile(ctx context.Context, studentID, documentID int64, actor careplan.StudentDocumentActor) (careplan.AccidentReportFile, error) {
	var file careplan.AccidentReportFile
	err := s.transaction(ctx, func(txCtx context.Context) error {
		document, err := s.findAccidentReport(txCtx, studentID, documentID, actor)
		if err != nil {
			return err
		}
		if err := s.recordSensitiveDownload(txCtx, studentID, actor, document); err != nil {
			return err
		}
		file.Document = document
		if document.AccidentSignatureID == nil {
			return nil
		}
		statement, err := careplan.AccidentReportStatement(document)
		if err != nil {
			return err
		}
		file.Signature, err = s.signatures.AccidentSignature(txCtx, studentID, *document.AccidentSignatureID, statement)
		if err != nil {
			return fmt.Errorf("read accident report signature: %w", err)
		}
		return nil
	})
	if err != nil {
		return careplan.AccidentReportFile{}, err
	}
	return file, nil
}

// findAccidentReport runs the per-child gate and the read permission and
// returns the live accident report.
func (s *StudentDocuments) findAccidentReport(ctx context.Context, studentID, documentID int64, actor careplan.StudentDocumentActor) (careplan.CareDocument, error) {
	if err := s.requireStudentAccess(ctx, studentID, actor); err != nil {
		return careplan.CareDocument{}, err
	}
	document, err := s.records.FindCareDocument(ctx, studentID, documentID, false)
	if err != nil {
		return careplan.CareDocument{}, err
	}
	if err := s.requireCategoryPermission(document.Category, actor); err != nil {
		return careplan.CareDocument{}, err
	}
	if document.AccidentReport == nil {
		return careplan.CareDocument{}, careplan.ErrAccidentReportNotFound
	}
	return document, nil
}

// findOpenAccidentReport is findAccidentReport for a report that may still
// change.
func (s *StudentDocuments) findOpenAccidentReport(ctx context.Context, studentID, documentID int64, actor careplan.StudentDocumentActor) (careplan.CareDocument, error) {
	document, err := s.findAccidentReport(ctx, studentID, documentID, actor)
	if err != nil {
		return careplan.CareDocument{}, err
	}
	if document.AccidentSignedAt != nil {
		return careplan.CareDocument{}, careplan.ErrAccidentReportSigned
	}
	return document, nil
}

// accidentReportFilename names the rendered PDF after the day of the
// accident in Berlin.
func accidentReportFilename(report careplan.AccidentReport) string {
	return "Unfallanzeige_" + calendar.DateFromTime(report.OccurredAt).String() + ".pdf"
}

// accidentAuditValue renders the history cell of a report: the label, the
// time of the accident and what happened to it.
func accidentAuditValue(document careplan.CareDocument, action string) string {
	value := documentAuditValue(document.Category, "Unfall am "+accidentStamp(document))
	if action != "" {
		value += " " + action
	}
	return value
}

func accidentStamp(document careplan.CareDocument) string {
	if document.AccidentReport == nil {
		return ""
	}
	return document.AccidentReport.OccurredAt.In(calendar.Berlin).Format("02.01.2006, 15:04") + " Uhr"
}
