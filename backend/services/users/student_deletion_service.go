package users

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

const (
	StudentDeletionReasonTestData       = "test_data"
	StudentDeletionReasonIncorrectEntry = "incorrect_entry"
	StudentDeletionReasonDuplicate      = "duplicate"
	StudentDeletionReasonPrivacyRequest = "privacy_request"
	StudentDeletionReasonGraduatePurge  = "graduate_purge"
	// StudentDeletionReasonRetentionExpired is the one deletion reason that
	// describes a NORMAL end of a record's life: the child left the OGS
	// regularly and the school's retention period for their data has run out
	// (#2487). It is only offered for a child whose care has ended — for a
	// child still in care there is no retention period running yet.
	StudentDeletionReasonRetentionExpired = "retention_expired"
)

type StudentDeletionPreview struct {
	ConfirmationName string
	Fingerprint      string
	Counts           userModels.StudentDeletionCounts
}

type StudentDeletionInput struct {
	StudentID           int64
	ActorAccountID      int64
	ExpectedFingerprint string
	ConfirmationName    string
	Reason              string
	Acknowledged        bool
}

type StudentDeletionResult struct {
	PhotoPath    string
	CompanionIDs []int64
	Counts       userModels.StudentDeletionCounts
}

type StudentDeletionService interface {
	Preview(ctx context.Context, studentID int64) (*StudentDeletionPreview, error)
	Delete(ctx context.Context, input StudentDeletionInput) (*StudentDeletionResult, error)
	AuditGraduatePurge(ctx context.Context, studentID, actorAccountID int64) error
}

type studentDeletionService struct {
	studentService StudentService
	studentRepo    userModels.StudentRepository
	personRepo     userModels.PersonRepository
	deletionRepo   userModels.StudentDeletionRepository
	transitionRepo educationModels.GradeTransitionRepository
	dataAuditRepo  auditModels.DataDeletionRepository
	auditRepo      auditModels.StudentDeletionRepository
	documentRepo   userModels.StudentDocumentRepository
	withdrawalRepo userModels.CareWithdrawalCompletionRepository
	txHandler      *modelBase.TxHandler
}

// WireStudentDocumentCleanup attaches the child-document repository to a
// deletion service so deleting a child also queues storage cleanup for their
// documents (#777). A service that does not support it is left untouched.
func WireStudentDocumentCleanup(svc StudentDeletionService, repo userModels.StudentDocumentRepository) {
	if setter, ok := svc.(interface {
		SetDocumentRepository(userModels.StudentDocumentRepository)
	}); ok {
		setter.SetDocumentRepository(repo)
	}
}

// WireStudentDeletionCareWithdrawals ensures every permanent deletion also
// resolves and redacts care-withdrawal history in the same transaction.
func WireStudentDeletionCareWithdrawals(svc StudentDeletionService, repo userModels.CareWithdrawalCompletionRepository) {
	setter, ok := svc.(interface {
		SetCareWithdrawalRepository(userModels.CareWithdrawalCompletionRepository)
	})
	if !ok {
		panic("student deletion service does not support care-withdrawal wiring")
	}
	setter.SetCareWithdrawalRepository(repo)
}

func (s *studentDeletionService) SetCareWithdrawalRepository(repo userModels.CareWithdrawalCompletionRepository) {
	s.withdrawalRepo = repo
}

// SetDocumentRepository wires the child-document repository so a deletion can
// queue storage cleanup intents for the child's documents (#777).
//
// This has to happen inside the deletion transaction. The document rows
// cascade away with the child, so queueing afterwards would race a crash and
// strand the bytes forever; queueing beforehand outside the transaction would
// make the scheduler delete the documents of a child whose deletion then
// failed. Nil keeps the pre-#777 behaviour for bare test services.
func (s *studentDeletionService) SetDocumentRepository(repo userModels.StudentDocumentRepository) {
	s.documentRepo = repo
}

// queueDocumentCleanup records an immediately-eligible cleanup intent for
// every document of the child whose bytes still exist. The intents carry no
// student foreign key, so they survive the cascade that removes the rows.
func (s *studentDeletionService) queueDocumentCleanup(ctx context.Context, studentID int64) error {
	if s.documentRepo == nil {
		return nil
	}
	docs, err := s.documentRepo.ListPendingFileCleanupByOwnerID(ctx, studentID)
	if err != nil {
		return err
	}
	for _, doc := range docs {
		cleanup := &userModels.StudentDocumentFileCleanup{}
		cleanup.OwnerID = studentID
		cleanup.FilenameStored = doc.FilenameStored
		cleanup.RetryAfter = time.Now()
		if err := s.documentRepo.QueueFileCleanup(ctx, cleanup); err != nil {
			return err
		}
	}
	return nil
}

func NewStudentDeletionService(
	studentService StudentService,
	studentRepo userModels.StudentRepository,
	personRepo userModels.PersonRepository,
	deletionRepo userModels.StudentDeletionRepository,
	transitionRepo educationModels.GradeTransitionRepository,
	dataAuditRepo auditModels.DataDeletionRepository,
	auditRepo auditModels.StudentDeletionRepository,
	db *bun.DB,
) StudentDeletionService {
	return &studentDeletionService{
		studentService: studentService,
		studentRepo:    studentRepo,
		personRepo:     personRepo,
		deletionRepo:   deletionRepo,
		transitionRepo: transitionRepo,
		dataAuditRepo:  dataAuditRepo,
		auditRepo:      auditRepo,
		txHandler:      modelBase.NewTxHandler(db),
	}
}

func (s *studentDeletionService) Preview(ctx context.Context, studentID int64) (*StudentDeletionPreview, error) {
	student, person, counts, err := s.loadPreview(ctx, studentID, false)
	if err != nil {
		return nil, err
	}
	if student.IsAlumnus() {
		return nil, ErrStudentDeletionAlumnus
	}
	return buildStudentDeletionPreview(student, person, counts)
}

func (s *studentDeletionService) Delete(ctx context.Context, input StudentDeletionInput) (*StudentDeletionResult, error) {
	if input.StudentID <= 0 || input.ActorAccountID <= 0 || strings.TrimSpace(input.ExpectedFingerprint) == "" {
		return nil, ErrStudentDeletionPreviewChanged
	}
	if !input.Acknowledged {
		return nil, ErrStudentDeletionNotAcknowledged
	}
	if !isValidStudentDeletionReason(input.Reason) {
		return nil, ErrStudentDeletionInvalidReason
	}

	result := new(StudentDeletionResult)
	err := s.txHandler.RunInTx(ctx, func(txCtx context.Context, _ bun.Tx) error {
		if err := s.studentService.LockCompanionGraph(txCtx, []int64{input.StudentID}, nil); err != nil {
			return err
		}
		if err := s.studentService.CheckCompanionTrim(txCtx, input.StudentID, nil); err != nil {
			return err
		}

		companionIDs, err := s.studentService.ListCompanionIDs(txCtx, input.StudentID)
		if err != nil {
			return err
		}
		student, person, counts, err := s.loadPreview(txCtx, input.StudentID, true)
		if err != nil {
			return err
		}
		if student.IsAlumnus() {
			return ErrStudentDeletionAlumnus
		}
		if input.Reason == StudentDeletionReasonRetentionExpired &&
			!student.CareEndedOn(timezone.TodayDate()) {
			return ErrStudentDeletionRetentionNotEnded
		}

		preview, err := buildStudentDeletionPreview(student, person, counts)
		if err != nil {
			return err
		}
		if !equalFingerprint(preview.Fingerprint, input.ExpectedFingerprint) {
			return ErrStudentDeletionPreviewChanged
		}
		if input.ConfirmationName != preview.ConfirmationName {
			return ErrStudentDeletionConfirmationMismatch
		}
		// Child rows can be removed by another transaction without touching the
		// locked student or person rows. Re-read every counted category directly
		// before the cascade so the confirmation and both audit records describe
		// this transaction's deletion, not an earlier snapshot.
		if err := s.ensureCountsUnchanged(txCtx, input.StudentID, *counts); err != nil {
			return err
		}

		deletedAssignments, err := s.deletionRepo.DeleteTimetableAssignments(txCtx, input.StudentID)
		if err != nil {
			return err
		}
		if deletedAssignments != int64(counts.TimetableAssignments) {
			return ErrStudentDeletionPreviewChanged
		}
		if err := s.deletionRepo.DeleteLegacyGuardianLinks(txCtx, student.PersonID); err != nil {
			return err
		}

		// Before the cascade: the document rows go with the child, the
		// queued intents do not, and they are what gets the bytes off disk.
		if err := s.queueDocumentCleanup(txCtx, input.StudentID); err != nil {
			return err
		}
		if s.withdrawalRepo != nil {
			if _, err := s.withdrawalRepo.MarkStudentDeleted(txCtx, input.StudentID, input.ActorAccountID, time.Now()); err != nil {
				return err
			}
		}
		if err := s.studentRepo.Delete(txCtx, input.StudentID); err != nil {
			return err
		}
		if err := s.transitionRepo.AnonymizeHistoryForStudent(txCtx, input.StudentID); err != nil {
			return err
		}
		anonymized, err := s.deletionRepo.AnonymizePersonIfUnchanged(txCtx, student.PersonID, person.UpdatedAt)
		if err != nil {
			return err
		}
		if !anonymized {
			return ErrStudentDeletionPreviewChanged
		}
		if err := s.createAudit(txCtx, input.StudentID, input.ActorAccountID, input.Reason, *counts, 1); err != nil {
			return err
		}

		if student.PhotoPath != nil {
			result.PhotoPath = *student.PhotoPath
		}
		result.CompanionIDs = companionIDs
		result.Counts = *counts
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// AuditGraduatePurge removes retained timetable assignments and records the
// dedicated Abgänge hard-delete path. The caller invokes it after taking the
// graph locks and before deleting the student row; its transaction therefore
// rolls both cleanup and audit back when the purge cannot finish.
func (s *studentDeletionService) AuditGraduatePurge(ctx context.Context, studentID, actorAccountID int64) error {
	if studentID <= 0 || actorAccountID <= 0 {
		return errors.New("graduate purge audit requires student and actor IDs")
	}
	// The purge path calculates its audit counts before the cascaded delete.
	// Locking the threads first serializes new messages and read cursors with
	// that snapshot, just as the regular deletion path does.
	if err := s.deletionRepo.LockMessageThreads(ctx, studentID); err != nil {
		return err
	}
	counts, err := s.deletionRepo.Preview(ctx, studentID)
	if err != nil {
		return err
	}
	if err := s.ensureCountsUnchanged(ctx, studentID, *counts); err != nil {
		return err
	}
	deletedAssignments, err := s.deletionRepo.DeleteTimetableAssignments(ctx, studentID)
	if err != nil {
		return err
	}
	if deletedAssignments != int64(counts.TimetableAssignments) {
		return ErrStudentDeletionPreviewChanged
	}
	if s.withdrawalRepo != nil {
		if _, err := s.withdrawalRepo.MarkStudentDeleted(ctx, studentID, actorAccountID, time.Now()); err != nil {
			return err
		}
	}
	return s.createAudit(ctx, studentID, actorAccountID, StudentDeletionReasonGraduatePurge, *counts, 2)
}

// ensureCountsUnchanged closes the gap between the locked preview and the
// delete. Not every child relation updates users.students when it is removed,
// so a student-row lock alone cannot make its earlier count authoritative.
func (s *studentDeletionService) ensureCountsUnchanged(
	ctx context.Context,
	studentID int64,
	expected userModels.StudentDeletionCounts,
) error {
	current, err := s.deletionRepo.Preview(ctx, studentID)
	if err != nil {
		return err
	}
	if *current != expected {
		return ErrStudentDeletionPreviewChanged
	}
	return nil
}

func (s *studentDeletionService) createAudit(
	ctx context.Context,
	studentID, actorAccountID int64,
	reason string,
	counts userModels.StudentDeletionCounts,
	primaryRowsDeleted int,
) error {
	if s.dataAuditRepo == nil || s.auditRepo == nil {
		return errors.New("student deletion audit repositories are not configured")
	}
	dataDeletion := auditModels.NewDataDeletion(
		studentID,
		auditModels.DeletionTypeManual,
		counts.Total()+primaryRowsDeleted,
		"account:"+strconv.FormatInt(actorAccountID, 10),
	)
	dataDeletion.DeletionReason = reason
	dataDeletion.SetMetadata("student_deletion", true)
	dataDeletion.SetMetadata("counts", counts)
	if err := s.dataAuditRepo.Create(ctx, dataDeletion); err != nil {
		return err
	}
	return s.auditRepo.Create(ctx, &auditModels.StudentDeletion{
		StudentID:      studentID,
		ActorAccountID: actorAccountID,
		Reason:         reason,
		Counts:         counts,
	})
}

func (s *studentDeletionService) loadPreview(
	ctx context.Context,
	studentID int64,
	lock bool,
) (*userModels.Student, *userModels.Person, *userModels.StudentDeletionCounts, error) {
	var (
		student *userModels.Student
		person  *userModels.Person
		err     error
	)
	if lock {
		student, err = s.studentService.GetByIDForUpdate(ctx, studentID)
	} else {
		student, err = s.studentRepo.FindByID(ctx, studentID)
	}
	if err != nil {
		return nil, nil, nil, err
	}
	if lock {
		if err := s.deletionRepo.LockMessageThreads(ctx, studentID); err != nil {
			return nil, nil, nil, err
		}
	}
	if lock {
		person, err = s.personRepo.FindByIDForUpdate(ctx, student.PersonID)
	} else {
		person, err = s.personRepo.FindByID(ctx, student.PersonID)
	}
	if err != nil {
		return nil, nil, nil, err
	}
	counts, err := s.deletionRepo.Preview(ctx, studentID)
	if err != nil {
		return nil, nil, nil, err
	}
	return student, person, counts, nil
}

func buildStudentDeletionPreview(
	student *userModels.Student,
	person *userModels.Person,
	counts *userModels.StudentDeletionCounts,
) (*StudentDeletionPreview, error) {
	if student == nil || person == nil || counts == nil {
		return nil, errors.New("student deletion preview is incomplete")
	}
	payload := struct {
		StudentUpdatedAt int64                            `json:"student_updated_at"`
		PersonUpdatedAt  int64                            `json:"person_updated_at"`
		Counts           userModels.StudentDeletionCounts `json:"counts"`
	}{
		StudentUpdatedAt: student.UpdatedAt.UnixNano(),
		PersonUpdatedAt:  person.UpdatedAt.UnixNano(),
		Counts:           *counts,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal student deletion preview: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return &StudentDeletionPreview{
		ConfirmationName: person.GetFullName(),
		Fingerprint:      hex.EncodeToString(sum[:]),
		Counts:           *counts,
	}, nil
}

func equalFingerprint(actual, expected string) bool {
	actualBytes, actualErr := hex.DecodeString(actual)
	expectedBytes, expectedErr := hex.DecodeString(strings.TrimSpace(expected))
	if actualErr != nil || expectedErr != nil || len(actualBytes) != len(expectedBytes) {
		return false
	}
	return subtle.ConstantTimeCompare(actualBytes, expectedBytes) == 1
}

func isValidStudentDeletionReason(reason string) bool {
	switch reason {
	case StudentDeletionReasonTestData,
		StudentDeletionReasonIncorrectEntry,
		StudentDeletionReasonDuplicate,
		StudentDeletionReasonPrivacyRequest,
		StudentDeletionReasonRetentionExpired:
		return true
	default:
		return false
	}
}

// ErrStudentDeletionRetentionNotEnded refuses the retention reason for a child
// who is still in care: nothing has started running out yet, and picking it
// would put a false statement into the deletion audit (#2487).
//
//nolint:staticcheck // ST1005: user-facing German message
var ErrStudentDeletionRetentionNotEnded = errors.New("Der Grund „Aufbewahrungsfrist abgelaufen“ gilt nur für Kinder, deren Betreuung beendet ist.")
