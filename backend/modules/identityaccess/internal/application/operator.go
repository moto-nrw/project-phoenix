package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Operator identity and refresh sessions are platform-wide rows. Every
// operation below runs through RunPlatform: it joins the administrative
// transaction the operator flows open for multi-statement work and otherwise
// executes on the root connection, exactly as the retained repositories did.

func (s *Service) FindOperator(ctx context.Context, id int64) (identityaccess.Operator, error) {
	operator, err := s.findOperatorByID(ctx, id)
	if err != nil {
		return identityaccess.Operator{}, fmt.Errorf("identity access: find operator: %w", MapError(err))
	}
	return identityaccess.Operator(operator), nil
}

func (s *Service) FindOperatorForUpdate(ctx context.Context, id int64) (identityaccess.Operator, error) {
	operator, err := s.findOperatorForUpdate(ctx, id)
	if err != nil {
		return identityaccess.Operator{}, fmt.Errorf("identity access: find operator for update: %w", MapError(err))
	}
	return identityaccess.Operator(operator), nil
}

func (s *Service) findOperatorByID(ctx context.Context, id int64) (domain.Operator, error) {
	return s.findOperator(ctx, "find_operator", id, false)
}

func (s *Service) findOperatorForUpdate(ctx context.Context, id int64) (domain.Operator, error) {
	return s.findOperator(ctx, "find_operator_for_update", id, true)
}

func (s *Service) findOperator(ctx context.Context, operation string, id int64, forUpdate bool) (result domain.Operator, err error) {
	run := s.tx.RunAdmin
	if forUpdate {
		run = func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }
	}
	err = s.run(ctx, run, operation, func(txCtx context.Context, stats *domain.OperationStats) error {
		if id <= 0 {
			return identityaccess.ErrOperatorNotFound
		}
		operator, found, queryStats, findErr := s.operators.FindOperator(txCtx, id, forUpdate)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return identityaccess.ErrOperatorNotFound
		}
		result = operator
		return nil
	})
	return result, err
}

func (s *Service) FindOperatorByEmail(ctx context.Context, email string) (identityaccess.Operator, error) {
	var result domain.Operator
	err := s.run(ctx, s.tx.RunAdmin, "find_operator_by_email", func(txCtx context.Context, stats *domain.OperationStats) error {
		normalized := strings.TrimSpace(email)
		if normalized == "" {
			return identityaccess.ErrOperatorNotFound
		}
		operator, found, queryStats, findErr := s.operators.FindOperatorByEmail(txCtx, normalized)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return identityaccess.ErrOperatorNotFound
		}
		result = operator
		return nil
	})
	if err != nil {
		return identityaccess.Operator{}, fmt.Errorf("identity access: find operator by email: %w", MapError(err))
	}
	return identityaccess.Operator(result), nil
}

func (s *Service) ListOperators(ctx context.Context) ([]identityaccess.Operator, error) {
	var result []domain.Operator
	err := s.run(ctx, s.tx.RunAdmin, "list_operators", func(txCtx context.Context, stats *domain.OperationStats) error {
		operators, queryStats, listErr := s.operators.ListOperators(txCtx)
		stats.Add(queryStats)
		if listErr != nil {
			return listErr
		}
		result = operators
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("identity access: list operators: %w", MapError(err))
	}
	operators := make([]identityaccess.Operator, 0, len(result))
	for _, operator := range result {
		operators = append(operators, identityaccess.Operator(operator))
	}
	return operators, nil
}

// CreateOperator validates and normalizes the address and display name
// before the insert.
func (s *Service) CreateOperator(ctx context.Context, operator identityaccess.Operator) (identityaccess.Operator, error) {
	var result domain.Operator
	err := s.run(ctx, s.tx.RunAdmin, "create_operator", func(txCtx context.Context, stats *domain.OperationStats) error {
		row := domain.Operator(operator)
		if validateErr := row.Validate(); validateErr != nil {
			return validateErr
		}
		stored, queryStats, insertErr := s.operators.InsertOperator(txCtx, row)
		stats.Add(queryStats)
		if insertErr != nil {
			return insertErr
		}
		result = stored
		return nil
	})
	if err != nil {
		return identityaccess.Operator{}, fmt.Errorf("identity access: create operator: %w", MapError(err))
	}
	return identityaccess.Operator(result), nil
}

// UpdateOperator validates and normalizes the address and display name and
// writes every mutable column.
func (s *Service) UpdateOperator(ctx context.Context, operator identityaccess.Operator) (identityaccess.Operator, error) {
	var result domain.Operator
	err := s.run(ctx, s.tx.RunAdmin, "update_operator", func(txCtx context.Context, stats *domain.OperationStats) error {
		row := domain.Operator(operator)
		if row.ID <= 0 {
			return identityaccess.ErrOperatorNotFound
		}
		if validateErr := row.Validate(); validateErr != nil {
			return validateErr
		}
		found, queryStats, updateErr := s.operators.UpdateOperator(txCtx, row)
		stats.Add(queryStats)
		if updateErr != nil {
			return updateErr
		}
		if !found {
			return identityaccess.ErrOperatorNotFound
		}
		result = row
		return nil
	})
	if err != nil {
		return identityaccess.Operator{}, fmt.Errorf("identity access: update operator: %w", MapError(err))
	}
	return identityaccess.Operator(result), nil
}

func (s *Service) DeleteOperator(ctx context.Context, id int64) error {
	err := s.run(ctx, s.tx.RunAdmin, "delete_operator", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.operators.DeleteOperator(txCtx, id)
		stats.Add(queryStats)
		return err
	})
	if err != nil {
		return fmt.Errorf("identity access: delete operator: %w", MapError(err))
	}
	return nil
}

// deleteDeactivatedOperators removes the accounts whose database-clock
// retention window has ended.
func (s *Service) deleteDeactivatedOperators(ctx context.Context) (result int, err error) {
	err = s.run(ctx, s.tx.RunAdmin, "delete_deactivated_operators", func(txCtx context.Context, stats *domain.OperationStats) error {
		deleted, queryStats, deleteErr := s.operators.DeleteDeactivatedOperators(txCtx)
		stats.Add(queryStats)
		if deleteErr != nil {
			return deleteErr
		}
		result = deleted
		return nil
	})
	return result, err
}

func (s *Service) RecordOperatorLogin(ctx context.Context, id int64) error {
	err := s.run(ctx, s.tx.RunAdmin, "record_operator_login", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.operators.RecordOperatorLogin(txCtx, id, time.Now())
		stats.Add(queryStats)
		return err
	})
	if err != nil {
		return fmt.Errorf("identity access: record operator login: %w", MapError(err))
	}
	return nil
}

func (s *Service) IncrementOperatorMFAAttempts(ctx context.Context, id int64, threshold int, lockout time.Duration) (identityaccess.OperatorMFAAttempts, error) {
	var result domain.OperatorMFAAttempts
	err := s.run(ctx, s.tx.RunAdmin, "increment_operator_mfa_attempts", func(txCtx context.Context, stats *domain.OperationStats) error {
		attempts, found, queryStats, incrementErr := s.operators.IncrementOperatorMFAAttempts(txCtx, id, threshold, time.Now().Add(lockout))
		stats.Add(queryStats)
		if incrementErr != nil {
			return incrementErr
		}
		if !found {
			return identityaccess.ErrOperatorNotFound
		}
		result = attempts
		return nil
	})
	if err != nil {
		return identityaccess.OperatorMFAAttempts{}, fmt.Errorf("identity access: increment operator mfa attempts: %w", MapError(err))
	}
	return identityaccess.OperatorMFAAttempts(result), nil
}

func (s *Service) ResetOperatorMFAAttempts(ctx context.Context, id int64) error {
	err := s.run(ctx, s.tx.RunAdmin, "reset_operator_mfa_attempts", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.operators.ResetOperatorMFAAttempts(txCtx, id)
		stats.Add(queryStats)
		return err
	})
	if err != nil {
		return fmt.Errorf("identity access: reset operator mfa attempts: %w", MapError(err))
	}
	return nil
}

func (s *Service) FindOperatorSessionForUpdate(ctx context.Context, token string) (identityaccess.OperatorSession, error) {
	session, err := s.findOperatorSessionForUpdate(ctx, token)
	if err != nil {
		return identityaccess.OperatorSession{}, fmt.Errorf("identity access: find operator session for update: %w", MapError(err))
	}
	return identityaccess.OperatorSession(session), nil
}

func (s *Service) findOperatorSessionForUpdate(ctx context.Context, token string) (result domain.OperatorSession, err error) {
	err = s.run(ctx, func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }, "find_operator_session_for_update", func(txCtx context.Context, stats *domain.OperationStats) error {
		if token == "" {
			return identityaccess.ErrOperatorSessionNotFound
		}
		session, found, queryStats, findErr := s.operators.FindOperatorSessionByToken(txCtx, token, true)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return identityaccess.ErrOperatorSessionNotFound
		}
		result = session
		return nil
	})
	return result, err
}

func (s *Service) LatestOperatorSessionInFamily(ctx context.Context, familyID string) (identityaccess.OperatorSession, error) {
	var result domain.OperatorSession
	err := s.run(ctx, s.tx.RunAdmin, "latest_operator_session_in_family", func(txCtx context.Context, stats *domain.OperationStats) error {
		if familyID == "" {
			return identityaccess.ErrOperatorSessionNotFound
		}
		session, found, queryStats, findErr := s.operators.LatestOperatorSessionInFamily(txCtx, familyID)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return identityaccess.ErrOperatorSessionNotFound
		}
		result = session
		return nil
	})
	if err != nil {
		return identityaccess.OperatorSession{}, fmt.Errorf("identity access: latest operator session in family: %w", MapError(err))
	}
	return identityaccess.OperatorSession(result), nil
}

func (s *Service) CreateOperatorSession(ctx context.Context, session identityaccess.OperatorSession) (identityaccess.OperatorSession, error) {
	var result domain.OperatorSession
	err := s.run(ctx, s.tx.RunAdmin, "create_operator_session", func(txCtx context.Context, stats *domain.OperationStats) error {
		row := domain.OperatorSession(session)
		if validateErr := row.Validate(); validateErr != nil {
			return validateErr
		}
		stored, queryStats, insertErr := s.operators.InsertOperatorSession(txCtx, row)
		stats.Add(queryStats)
		if insertErr != nil {
			return insertErr
		}
		result = stored
		return nil
	})
	if err != nil {
		return identityaccess.OperatorSession{}, fmt.Errorf("identity access: create operator session: %w", MapError(err))
	}
	return identityaccess.OperatorSession(result), nil
}

// MarkOperatorSessionRotated records the hand-off exactly once: a session that
// was already rotated (or never existed) is reported, never silently
// re-rotated, because a second hand-off is the replay signal the refresh
// flow revokes the whole family on.
func (s *Service) MarkOperatorSessionRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) error {
	err := s.run(ctx, s.tx.RunAdmin, "mark_operator_session_rotated", func(txCtx context.Context, stats *domain.OperationStats) error {
		rotated, queryStats, err := s.operators.MarkOperatorSessionRotated(txCtx, id, replacementToken, recoveryProofHash, rotatedAt)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !rotated {
			return identityaccess.ErrOperatorSessionRotated
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("identity access: mark operator session rotated: %w", MapError(err))
	}
	return nil
}

func (s *Service) DeleteExpiredRotatedOperatorSessions(ctx context.Context, familyID string, now time.Time) error {
	err := s.run(ctx, s.tx.RunAdmin, "delete_expired_rotated_operator_sessions", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.operators.DeleteExpiredRotatedOperatorSessions(txCtx, familyID, now)
		stats.Add(queryStats)
		return err
	})
	if err != nil {
		return fmt.Errorf("identity access: delete expired rotated operator sessions: %w", MapError(err))
	}
	return nil
}

func (s *Service) DeleteOperatorSession(ctx context.Context, id int64) error {
	err := s.run(ctx, s.tx.RunAdmin, "delete_operator_session", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.operators.DeleteOperatorSession(txCtx, id)
		stats.Add(queryStats)
		return err
	})
	if err != nil {
		return fmt.Errorf("identity access: delete operator session: %w", MapError(err))
	}
	return nil
}

func (s *Service) RevokeOperatorSessions(ctx context.Context, operatorID int64) ([]identityaccess.OperatorSession, error) {
	var result []domain.OperatorSession
	err := s.run(ctx, s.tx.RunAdmin, "revoke_operator_sessions", func(txCtx context.Context, stats *domain.OperationStats) error {
		deleted, queryStats, deleteErr := s.operators.DeleteOperatorSessionsByOperator(txCtx, operatorID)
		stats.Add(queryStats)
		if deleteErr != nil {
			return deleteErr
		}
		result = deleted
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("identity access: revoke operator sessions: %w", MapError(err))
	}
	return operatorSessions(result), nil
}

func (s *Service) RevokeOperatorSessionFamily(ctx context.Context, familyID string) ([]identityaccess.OperatorSession, error) {
	var result []domain.OperatorSession
	err := s.run(ctx, s.tx.RunAdmin, "revoke_operator_session_family", func(txCtx context.Context, stats *domain.OperationStats) error {
		deleted, queryStats, deleteErr := s.operators.DeleteOperatorSessionsByFamily(txCtx, familyID)
		stats.Add(queryStats)
		if deleteErr != nil {
			return deleteErr
		}
		result = deleted
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("identity access: revoke operator session family: %w", MapError(err))
	}
	return operatorSessions(result), nil
}

func (s *Service) DeleteExpiredOperatorSessions(ctx context.Context, now time.Time) (int, error) {
	var result int
	err := s.run(ctx, s.tx.RunAdmin, "delete_expired_operator_sessions", func(txCtx context.Context, stats *domain.OperationStats) error {
		deleted, queryStats, deleteErr := s.operators.DeleteExpiredOperatorSessions(txCtx, now)
		stats.Add(queryStats)
		if deleteErr != nil {
			return deleteErr
		}
		result = deleted
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("identity access: delete expired operator sessions: %w", MapError(err))
	}
	return result, nil
}

func operatorSessions(values []domain.OperatorSession) []identityaccess.OperatorSession {
	if values == nil {
		return nil
	}
	result := make([]identityaccess.OperatorSession, 0, len(values))
	for _, value := range values {
		result = append(result, identityaccess.OperatorSession(value))
	}
	return result
}
