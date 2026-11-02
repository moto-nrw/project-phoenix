package contracttest_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// Accident reports (#4771): an unfallanzeige document carries the recorded
// accident. Reading needs student_accidents:read or :manage, every write
// needs student_accidents:manage; a signed report never changes again, a
// correction is a new report and the old one stays; every write leaves a
// change-history row and the PDF download a data-access log row.

func newAccidentReports(t *testing.T, db *bun.DB) careplan.StudentAccidentReports {
	t.Helper()
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	audit := owners.NewTestAuditPersistence(testpkg.NewDatabase(t, db), nil)
	reports, err := api.NewStudentAccidentReportsTestModule(testpkg.NewDatabase(t, db), repos.CarePlan(), reposPeople.Student,
		stubDocumentUserContext{staff: &peopleValues.Staff{}}, audit.StudentFieldEdit, audit.DataAccessLog)
	require.NoError(t, err)
	return reports
}

func sampleAccidentReport() careplan.AccidentReport {
	notified := time.Date(2026, 10, 20, 10, 15, 0, 0, time.UTC)
	return careplan.AccidentReport{
		OccurredAt: time.Date(2026, 10, 20, 9, 40, 0, 0, time.UTC), Location: "Schulhof, Klettergerüst",
		Description: "Beim Klettern abgerutscht und auf den linken Arm gefallen.", InjuredBodyParts: "Linker Unterarm",
		InjuryKind: "Prellung", Witnesses: "Frau Berger (Aufsicht)", FirstAid: "Gekühlt, Arm ruhiggestellt",
		DoctorVisited: true, Doctor: "Kinderarztpraxis Dr. Lindner", ParentsNotified: "Mutter, telefonisch",
		ParentsNotifiedAt: &notified, Supervisor: "Frau Berger",
	}
}

// signaturePNG is a small drawn signature: a dark diagonal on transparency.
func signaturePNG(t *testing.T) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 60, 20))
	for x := 0; x < 60; x++ {
		img.SetNRGBA(x, x/3, color.NRGBA{R: 20, G: 30, B: 80, A: 255})
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestStudentAccidentReports_LifecycleAndImmutability(t *testing.T) {
	t.Parallel()

	s := newStudentDocumentScenario(t)
	reports := newAccidentReports(t, s.db)
	leader := s.actor("student_accidents:read", "student_accidents:manage")

	report, err := reports.CreateStudentAccidentReport(s.ctx, s.studentID, sampleAccidentReport(), leader)
	require.NoError(t, err)
	assert.Equal(t, careplan.StudentDocumentCategoryUnfallanzeige, report.Category)
	assert.Equal(t, careplan.AccidentReportStatusOpen, careplan.AccidentReportStatus(report))
	assert.Empty(t, report.FilenameStored, "a report without paper copy stores no file")
	assert.Equal(t, "Unfallanzeige_2026-10-20.pdf", report.FilenameDisplay)

	changed := sampleAccidentReport()
	changed.InjuryKind = "Verstauchung"
	report, err = reports.UpdateStudentAccidentReport(s.ctx, s.studentID, report.ID, changed, leader)
	require.NoError(t, err)
	assert.Equal(t, "Verstauchung", report.AccidentReport.InjuryKind)

	_, err = reports.CorrectStudentAccidentReport(s.ctx, s.studentID, report.ID, changed, leader)
	require.ErrorIs(t, err, careplan.ErrAccidentReportNotSigned, "an open report is changed directly")

	signed, err := reports.SignStudentAccidentReport(s.ctx, s.studentID, report.ID, careplan.AccidentSignatureInput{
		Method: "drawn", ImagePNG: signaturePNG(t),
	}, leader)
	require.NoError(t, err)
	require.NotNil(t, signed.AccidentSignatureID)
	assert.Equal(t, careplan.AccidentReportStatusSigned, careplan.AccidentReportStatus(signed))

	_, err = reports.UpdateStudentAccidentReport(s.ctx, s.studentID, report.ID, sampleAccidentReport(), leader)
	require.ErrorIs(t, err, careplan.ErrAccidentReportSigned)
	_, err = reports.SignStudentAccidentReport(s.ctx, s.studentID, report.ID, careplan.AccidentSignatureInput{
		Method: "drawn", ImagePNG: signaturePNG(t),
	}, leader)
	require.ErrorIs(t, err, careplan.ErrAccidentReportSigned)
	_, err = s.svc.DeleteStudentDocument(s.ctx, s.studentID, report.ID, leader)
	require.ErrorIs(t, err, careplan.ErrAccidentReportSigned, "a signed report is a record that stays")

	// The database refuses a change of a signed report even past the service.
	_, err = s.db.NewRaw(`UPDATE users.student_documents SET accident_report = '{}'::jsonb WHERE id = ?`, report.ID).Exec(context.Background())
	require.Error(t, err)

	correction, err := reports.CorrectStudentAccidentReport(s.ctx, s.studentID, report.ID, sampleAccidentReport(), leader)
	require.NoError(t, err)
	require.NotNil(t, correction.AccidentSupersedesID)
	assert.Equal(t, report.ID, *correction.AccidentSupersedesID)
	assert.Equal(t, careplan.AccidentReportStatusOpen, careplan.AccidentReportStatus(correction))
	_, err = reports.CorrectStudentAccidentReport(s.ctx, s.studentID, report.ID, sampleAccidentReport(), leader)
	require.ErrorIs(t, err, careplan.ErrAccidentReportSuperseded)

	docs, _, err := s.svc.ListStudentDocuments(s.ctx, s.studentID, careplan.StudentDocumentCategoryUnfallanzeige, leader)
	require.NoError(t, err)
	assert.Len(t, docs, 2, "the signed version stays next to its correction")

	file, err := reports.ResolveStudentAccidentReportFile(s.ctx, s.studentID, report.ID, leader)
	require.NoError(t, err)
	require.NotNil(t, file.Signature)
	assert.True(t, file.Signature.Intact, "the stored report still matches its signature")
	assert.Equal(t, "Test Person", file.Signature.SignerName)

	// Erfassen, Ändern, Unterschreiben, Korrigieren in the history; the
	// download in the data-access log.
	assert.Len(t, s.auditRows(t), 4)
	logs := s.accessLogRows(t)
	require.Len(t, logs, 1)
	assert.Equal(t, report.ID, int64(logs[0].Metadata["document_id"].(float64)))
}

func TestStudentAccidentReports_PaperCopySigns(t *testing.T) {
	t.Parallel()

	s := newStudentDocumentScenario(t)
	reports := newAccidentReports(t, s.db)
	leader := s.actor("student_accidents:read", "student_accidents:manage")

	report, err := reports.CreateStudentAccidentReport(s.ctx, s.studentID, sampleAccidentReport(), leader)
	require.NoError(t, err)
	require.NoError(t, reports.AuthorizeAccidentPaperCopy(s.ctx, s.studentID, report.ID, leader))

	stored := "unfallanzeige-papier-" + time.Now().Format("150405.000000000") + ".pdf"
	require.NoError(t, s.svc.QueueStudentDocumentFileCleanup(s.ctx, s.studentID, stored))
	signed, err := reports.AttachAccidentPaperCopy(s.ctx, careplan.AccidentPaperCopyInput{
		StudentID: s.studentID, DocumentID: report.ID, FilenameDisplay: "Unfallanzeige unterschrieben.pdf",
		FilenameStored: stored, SizeBytes: 1024, ContentType: "application/pdf",
	}, leader)
	require.NoError(t, err)
	assert.Equal(t, careplan.AccidentReportStatusSigned, careplan.AccidentReportStatus(signed))
	assert.Nil(t, signed.AccidentSignatureID)
	assert.Equal(t, stored, signed.FilenameStored)

	require.ErrorIs(t, reports.AuthorizeAccidentPaperCopy(s.ctx, s.studentID, report.ID, leader), careplan.ErrAccidentReportSigned)
}

func TestStudentAccidentReports_PermissionsAndValidation(t *testing.T) {
	t.Parallel()

	s := newStudentDocumentScenario(t)
	reports := newAccidentReports(t, s.db)
	leader := s.actor("student_accidents:read", "student_accidents:manage")
	reader := s.actor("student_accidents:read")
	manager := s.actor("student_accidents:manage")
	office := s.actor("users:update")

	_, err := reports.CreateStudentAccidentReport(s.ctx, s.studentID, sampleAccidentReport(), office)
	require.ErrorIs(t, err, careplan.ErrStudentDocumentForbidden)
	_, err = reports.CreateStudentAccidentReport(s.ctx, s.studentID, sampleAccidentReport(), reader)
	require.ErrorIs(t, err, careplan.ErrStudentDocumentForbidden, "reading does not allow recording")

	incomplete := sampleAccidentReport()
	incomplete.Description = "  "
	_, err = reports.CreateStudentAccidentReport(s.ctx, s.studentID, incomplete, leader)
	require.ErrorIs(t, err, careplan.ErrAccidentReportInvalid)

	report, err := reports.CreateStudentAccidentReport(s.ctx, s.studentID, sampleAccidentReport(), leader)
	require.NoError(t, err)

	// Without either accident permission the category does not appear at all.
	docs, visible, err := s.svc.ListStudentDocuments(s.ctx, s.studentID, "", office)
	require.NoError(t, err)
	assert.NotContains(t, visible, careplan.StudentDocumentCategoryUnfallanzeige)
	assert.NotContains(t, studentDocumentCategoriesOf(docs), careplan.StudentDocumentCategoryUnfallanzeige)
	_, err = reports.ResolveStudentAccidentReportFile(s.ctx, s.studentID, report.ID, office)
	require.ErrorIs(t, err, careplan.ErrStudentDocumentForbidden)

	_, visible, err = s.svc.ListStudentDocuments(s.ctx, s.studentID, "", reader)
	require.NoError(t, err)
	assert.Equal(t, []string{careplan.StudentDocumentCategoryUnfallanzeige}, visible)
	_, err = reports.ResolveStudentAccidentReportFile(s.ctx, s.studentID, report.ID, reader)
	require.NoError(t, err)

	docs, visible, err = s.svc.ListStudentDocuments(s.ctx, s.studentID, "", manager)
	require.NoError(t, err)
	assert.Equal(t, []string{careplan.StudentDocumentCategoryUnfallanzeige}, visible)
	assert.Equal(t, []string{careplan.StudentDocumentCategoryUnfallanzeige}, studentDocumentCategoriesOf(docs))
	_, err = reports.ResolveStudentAccidentReportFile(s.ctx, s.studentID, report.ID, manager)
	require.NoError(t, err, "managing an accident report includes opening it")

	_, err = reports.SignStudentAccidentReport(s.ctx, s.studentID, report.ID, careplan.AccidentSignatureInput{
		Method: "drawn", ImagePNG: base64.StdEncoding.EncodeToString([]byte("not a png")),
	}, leader)
	require.ErrorIs(t, err, careplan.ErrAccidentSignatureInvalid)
}

// Another school sees neither the report nor its child (School ID, RLS).
func TestStudentAccidentReports_TenantIsolation(t *testing.T) {
	t.Parallel()

	s := newStudentDocumentScenario(t)
	reports := newAccidentReports(t, s.db)
	leader := s.actor("student_accidents:read", "student_accidents:manage")
	report, err := reports.CreateStudentAccidentReport(s.ctx, s.studentID, sampleAccidentReport(), leader)
	require.NoError(t, err)

	otherSchool, _ := testpkg.CreateTestTenant(t, s.db)
	otherCtx := testpkg.TenantContext(otherSchool)
	_, err = reports.ResolveStudentAccidentReportFile(otherCtx, s.studentID, report.ID, leader)
	require.Error(t, err)
	_, err = reports.UpdateStudentAccidentReport(otherCtx, s.studentID, report.ID, sampleAccidentReport(), leader)
	require.Error(t, err)
}

// A report is recorded, never uploaded as a loose file, and a report
// without a paper copy has no file the document download could serve or log.
func TestStudentAccidentReports_StayOffThePlainDocumentPaths(t *testing.T) {
	t.Parallel()

	s := newStudentDocumentScenario(t)
	reports := newAccidentReports(t, s.db)
	leader := s.actor("student_accidents:read", "student_accidents:manage")

	require.ErrorIs(t, s.svc.AuthorizeStudentDocumentUpload(s.ctx, s.studentID, careplan.StudentDocumentCategoryUnfallanzeige, leader),
		careplan.ErrStudentDocumentInvalid)
	_, err := s.svc.CreateStudentDocument(s.ctx, s.input(careplan.StudentDocumentCategoryUnfallanzeige), leader)
	require.ErrorIs(t, err, careplan.ErrStudentDocumentInvalid)

	report, err := reports.CreateStudentAccidentReport(s.ctx, s.studentID, sampleAccidentReport(), leader)
	require.NoError(t, err)
	_, err = s.svc.ResolveStudentDocumentDownload(s.ctx, s.studentID, report.ID, leader)
	require.ErrorIs(t, err, careplan.ErrCareDocumentNotFound)
	assert.Empty(t, s.accessLogRows(t), "nothing served, nothing logged")

	future := sampleAccidentReport()
	future.OccurredAt = time.Now().Add(time.Hour)
	_, err = reports.CreateStudentAccidentReport(s.ctx, s.studentID, future, leader)
	require.ErrorIs(t, err, careplan.ErrAccidentReportInvalid)
}
