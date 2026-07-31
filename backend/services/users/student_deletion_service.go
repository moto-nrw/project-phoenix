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
	txHandler      *modelBase.TxHandler
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
	deletedAssignments, err := s.deletionRepo.DeleteTimetableAssignments(ctx, studentID)
	if err != nil {
		return err
	}
	if deletedAssignments != int64(counts.TimetableAssignments) {
		return ErrStudentDeletionPreviewChanged
	}
	return s.createAudit(ctx, studentID, actorAccountID, StudentDeletionReasonGraduatePurge, *counts, 2)
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
		StudentDeletionReasonPrivacyRequest:
		return true
	default:
		return false
	}
}
