package enrollment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

var (
	ErrEnrollmentDeletionNotFound      = errors.New("enrollment deletion target not found")
	ErrEnrollmentDeletionInvalidReason = errors.New("deletion reason must contain between 3 and 500 characters")
	ErrEnrollmentDeletionNotAllowed    = errors.New("enrollment child cannot be deleted in its current status")
	ErrEnrollmentDeletionStudentExists = errors.New("enrollment deletion blocked because a created student still exists")
)

type EnrollmentDeletionService interface {
	PreviewRequest(ctx context.Context, requestID int64) (*enrollmentModels.DeletionImpact, error)
	PreviewChild(ctx context.Context, requestID, childID int64) (*enrollmentModels.DeletionImpact, error)
	DeleteRequest(ctx context.Context, requestID, actorAccountID int64, reason string) (*enrollmentModels.DeletionImpact, error)
	DeleteChild(ctx context.Context, requestID, childID, actorAccountID int64, reason string) (*enrollmentModels.DeletionImpact, error)
}

type DeletionChildren interface {
	DeleteRequestTree(context.Context, int64) error
	DeleteRequestChildTree(context.Context, int64, int64) error
	RequestChildrenReader
	ChildByID(context.Context, int64) (*capability.RequestChild, error)
}

type enrollmentDeletionService struct {
	requests RequestIDReader
	children DeletionChildren
	deletion DeletionPreview
	audit    auditModels.EnrollmentDeletionRepository
	tx       *tenant.TransactionRunner
	logger   *slog.Logger
	delivery EnrollmentDeletionDelivery
}

type EnrollmentDeletionDelivery interface {
	CountRelatedEmails(context.Context, string, int64) (int, error)
	CancelRelatedEmails(context.Context, string, int64, string) (int64, error)
}

const enrollmentRequestDeliveryType = "enrollment_request"

func NewEnrollmentDeletionService(
	requests RequestIDReader,
	children DeletionChildren,
	deletion DeletionPreview,
	audit auditModels.EnrollmentDeletionRepository,
	db *bun.DB,
	logger *slog.Logger,
	delivery EnrollmentDeletionDelivery,
) EnrollmentDeletionService {
	if logger == nil {
		logger = slog.Default()
	}
	return &enrollmentDeletionService{
		requests: requests,
		children: children,
		deletion: deletion,
		audit:    audit,
		tx:       tenant.NewTransactionRunner(),
		logger:   logger,
		delivery: delivery,
	}
}

func (s *enrollmentDeletionService) PreviewRequest(ctx context.Context, requestID int64) (*enrollmentModels.DeletionImpact, error) {
	if err := s.validateConfigured(); err != nil {
		return nil, err
	}
	impact, err := s.deletion.PreviewRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if impact == nil || impact.Counts.Requests != 1 {
		return nil, ErrEnrollmentDeletionNotFound
	}
	if err := s.addDeliveryCount(ctx, impact); err != nil {
		return nil, err
	}
	return impact, nil
}

func (s *enrollmentDeletionService) PreviewChild(ctx context.Context, requestID, childID int64) (*enrollmentModels.DeletionImpact, error) {
	if err := s.validateConfigured(); err != nil {
		return nil, err
	}
	impact, err := s.deletion.PreviewChild(ctx, requestID, childID)
	if err != nil {
		return nil, err
	}
	if impact == nil || impact.Counts.RequestChildren != 1 {
		return nil, ErrEnrollmentDeletionNotFound
	}
	child, err := s.children.ChildByID(ctx, childID)
	if err != nil {
		return nil, fmt.Errorf("load enrollment child for deletion preview: %w", err)
	}
	if child == nil || child.RequestID != requestID {
		return nil, ErrEnrollmentDeletionNotFound
	}
	if impact.DeletesRequest {
		if err := s.addDeliveryCount(ctx, impact); err != nil {
			return nil, err
		}
	}
	if err := validateDeletableEnrollmentChild(child); err != nil {
		return nil, err
	}
	return impact, nil
}

func (s *enrollmentDeletionService) DeleteRequest(ctx context.Context, requestID, actorAccountID int64, reason string) (*enrollmentModels.DeletionImpact, error) {
	reason, err := validateEnrollmentDeletionInput(actorAccountID, reason)
	if err != nil {
		return nil, err
	}
	if err = s.validateConfigured(); err != nil {
		return nil, err
	}
	var impact *enrollmentModels.DeletionImpact
	err = s.tx.RunInTx(ctx, func(txCtx context.Context) error {
		if lockErr := s.lockRequestForDeletion(txCtx, requestID); lockErr != nil {
			return lockErr
		}
		if _, lockErr := s.children.ChildrenForRequest(txCtx, requestID, true); lockErr != nil {
			return fmt.Errorf("lock enrollment request children for deletion: %w", lockErr)
		}
		impact, err = s.deletion.PreviewRequest(txCtx, requestID)
		if err != nil {
			return err
		}
		if impact == nil || impact.Counts.Requests != 1 {
			return ErrEnrollmentDeletionNotFound
		}
		if len(impact.BlockingStudentIDs) > 0 {
			return ErrEnrollmentDeletionStudentExists
		}
		if err = s.addDeliveryCount(txCtx, impact); err != nil {
			return err
		}
		if _, err = s.delivery.CancelRelatedEmails(txCtx, enrollmentRequestDeliveryType, requestID, "enrollment request deleted"); err != nil {
			return fmt.Errorf("cancel enrollment delivery intents: %w", err)
		}
		if err = s.children.DeleteRequestTree(txCtx, requestID); err != nil {
			return err
		}
		return s.auditDeletion(txCtx, impact, actorAccountID, reason, auditModels.EnrollmentDeletionScopeRequest)
	})
	if err != nil {
		return nil, err
	}
	s.logger.InfoContext(ctx, "enrollment request deleted",
		slog.Int64("request_id", requestID),
		slog.Int64("actor_account_id", actorAccountID),
		slog.Int("records_affected", impact.Counts.Total()))
	return impact, nil
}

func (s *enrollmentDeletionService) DeleteChild(ctx context.Context, requestID, childID, actorAccountID int64, reason string) (*enrollmentModels.DeletionImpact, error) {
	reason, err := validateEnrollmentDeletionInput(actorAccountID, reason)
	if err != nil {
		return nil, err
	}
	if err = s.validateConfigured(); err != nil {
		return nil, err
	}
	var impact *enrollmentModels.DeletionImpact
	err = s.tx.RunInTx(ctx, func(txCtx context.Context) error {
		if lockErr := s.lockRequestForDeletion(txCtx, requestID); lockErr != nil {
			return lockErr
		}
		children, lockErr := s.children.ChildrenForRequest(txCtx, requestID, true)
		if lockErr != nil {
			return fmt.Errorf("lock enrollment request children for deletion: %w", lockErr)
		}
		child := deletionRequestChildByID(children, childID)
		if child == nil {
			return ErrEnrollmentDeletionNotFound
		}
		if validationErr := validateDeletableEnrollmentChild(child); validationErr != nil {
			return validationErr
		}
		impact, err = s.deletion.PreviewChild(txCtx, requestID, childID)
		if err != nil {
			return err
		}
		if impact == nil || impact.Counts.RequestChildren != 1 {
			return ErrEnrollmentDeletionNotFound
		}
		if len(impact.BlockingStudentIDs) > 0 {
			return ErrEnrollmentDeletionStudentExists
		}
		if impact.DeletesRequest {
			if err = s.addDeliveryCount(txCtx, impact); err != nil {
				return err
			}
			if _, err = s.delivery.CancelRelatedEmails(txCtx, enrollmentRequestDeliveryType, requestID, "enrollment request deleted"); err != nil {
				return fmt.Errorf("cancel enrollment delivery intents: %w", err)
			}
			err = s.children.DeleteRequestTree(txCtx, requestID)
		} else {
			err = s.children.DeleteRequestChildTree(txCtx, requestID, childID)
		}
		if err != nil {
			return err
		}
		return s.auditDeletion(txCtx, impact, actorAccountID, reason, auditModels.EnrollmentDeletionScopeChild)
	})
	if err != nil {
		return nil, err
	}
	s.logger.InfoContext(ctx, "enrollment child deleted",
		slog.Int64("request_id", requestID),
		slog.Int64("child_id", childID),
		slog.Int64("actor_account_id", actorAccountID),
		slog.Bool("request_deleted", impact.DeletesRequest),
		slog.Int("records_affected", impact.Counts.Total()))
	return impact, nil
}

func (s *enrollmentDeletionService) lockRequestForDeletion(ctx context.Context, requestID int64) error {
	_, err := s.requests.RequestByID(ctx, requestID, true)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrEnrollmentDeletionNotFound
	}
	if err != nil {
		return fmt.Errorf("lock enrollment request for deletion: %w", err)
	}
	return nil
}

func (s *enrollmentDeletionService) auditDeletion(ctx context.Context, impact *enrollmentModels.DeletionImpact, actorAccountID int64, reason, scope string) error {
	event := &auditModels.EnrollmentDeletion{
		RequestID:      impact.RequestID,
		ChildID:        impact.ChildID,
		ActorAccountID: &actorAccountID,
		ActorType:      auditModels.EnrollmentDeletionActorAdmin,
		Scope:          scope,
		Reason:         reason,
		Counts:         auditDeletionCounts(impact.Counts),
		DeletedAt:      time.Now(),
	}
	if err := s.audit.Create(ctx, event); err != nil {
		return fmt.Errorf("audit enrollment deletion: %w", err)
	}
	return nil
}

func auditDeletionCounts(counts enrollmentModels.DeletionCounts) auditModels.EnrollmentDeletionCounts {
	return auditModels.EnrollmentDeletionCounts{
		Requests: counts.Requests, RequestChildren: counts.RequestChildren,
		RequestChildOfferings: counts.RequestChildOfferings, RequestGuardians: counts.RequestGuardians,
		ChangeRequests: counts.ChangeRequests, ChangeRequestMessages: counts.ChangeRequestMessages,
		LateInvites: counts.LateInvites, OfferingAdjustments: counts.OfferingAdjustments,
		EmailOutbox: counts.EmailOutbox, RolloverLinksCleared: counts.RolloverLinksCleared,
		StudentSourceLinksCleared: counts.StudentSourceLinksCleared,
	}
}

func (s *enrollmentDeletionService) validateConfigured() error {
	if s.requests == nil || s.children == nil || s.deletion == nil || s.audit == nil || s.tx == nil || s.delivery == nil {
		return errors.New("enrollment deletion service is not configured")
	}
	return nil
}

func (s *enrollmentDeletionService) addDeliveryCount(ctx context.Context, impact *enrollmentModels.DeletionImpact) error {
	count, err := s.delivery.CountRelatedEmails(ctx, enrollmentRequestDeliveryType, impact.RequestID)
	if err != nil {
		return fmt.Errorf("count enrollment delivery intents: %w", err)
	}
	impact.Counts.EmailOutbox = count
	return nil
}

func validateEnrollmentDeletionInput(actorAccountID int64, reason string) (string, error) {
	if actorAccountID <= 0 {
		return "", errors.New("actor account id is required for enrollment deletion")
	}
	reason = strings.TrimSpace(reason)
	if len([]rune(reason)) < 3 || len([]rune(reason)) > 500 {
		return "", ErrEnrollmentDeletionInvalidReason
	}
	return reason, nil
}

func validateDeletableEnrollmentChild(child *capability.RequestChild) error {
	if child == nil {
		return ErrEnrollmentDeletionNotFound
	}
	if child.CreatedStudentID != nil && *child.CreatedStudentID > 0 {
		return ErrEnrollmentDeletionStudentExists
	}
	switch child.Status {
	case enrollmentModels.ChildStatusRejected,
		enrollmentModels.ChildStatusWithdrawn,
		enrollmentModels.ChildStatusApproved:
		return nil
	default:
		return ErrEnrollmentDeletionNotAllowed
	}
}

func deletionRequestChildByID(children []*capability.RequestChild, childID int64) *capability.RequestChild {
	for _, child := range children {
		if child != nil && child.ID == childID {
			return child
		}
	}
	return nil
}
