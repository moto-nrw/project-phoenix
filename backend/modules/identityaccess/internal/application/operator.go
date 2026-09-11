package application

import (
	"context"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Operator identity and refresh sessions are platform-wide rows. Every
// operation below runs through RunPlatform: it joins the administrative
// transaction the operator flows open for multi-statement work and otherwise
// executes on the root connection, exactly as the retained repositories did.

func (s *Service) FindOperator(ctx context.Context, id int64) (domain.Operator, error) {
	return s.findOperator(ctx, "find_operator", id, false)
}

func (s *Service) FindOperatorForUpdate(ctx context.Context, id int64) (domain.Operator, error) {
	return s.findOperator(ctx, "find_operator_for_update", id, true)
}

func (s *Service) findOperator(ctx context.Context, operation string, id int64, forUpdate bool) (result domain.Operator, err error) {
	err = s.run(ctx, s.tx.RunPlatform, operation, func(txCtx context.Context, stats *domain.OperationStats) error {
		if id <= 0 {
			return domain.ErrOperatorNotFound
		}
		operator, found, queryStats, findErr := s.operators.FindOperator(txCtx, id, forUpdate)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrOperatorNotFound
		}
		result = operator
		return nil
	})
	return result, err
}

func (s *Service) FindOperatorByEmail(ctx context.Context, email string) (result domain.Operator, err error) {
	err = s.run(ctx, s.tx.RunPlatform, "find_operator_by_email", func(txCtx context.Context, stats *domain.OperationStats) error {
		normalized := strings.TrimSpace(email)
		if normalized == "" {
			return domain.ErrOperatorNotFound
		}
		operator, found, queryStats, findErr := s.operators.FindOperatorByEmail(txCtx, normalized)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrOperatorNotFound
		}
		result = operator
		return nil
	})
	return result, err
}

func (s *Service) ListOperators(ctx context.Context) (result []domain.Operator, err error) {
	err = s.run(ctx, s.tx.RunPlatform, "list_operators", func(txCtx context.Context, stats *domain.OperationStats) error {
		operators, queryStats, listErr := s.operators.ListOperators(txCtx)
		stats.Add(queryStats)
		if listErr != nil {
			return listErr
		}
		result = operators
		return nil
	})
	return result, err
}

func (s *Service) CreateOperator(ctx context.Context, operator domain.Operator) (result domain.Operator, err error) {
	err = s.run(ctx, s.tx.RunPlatform, "create_operator", func(txCtx context.Context, stats *domain.OperationStats) error {
		if validateErr := operator.Validate(); validateErr != nil {
			return validateErr
		}
		stored, queryStats, insertErr := s.operators.InsertOperator(txCtx, operator)
		stats.Add(queryStats)
		if insertErr != nil {
			return insertErr
		}
		result = stored
		return nil
	})
	return result, err
}

func (s *Service) UpdateOperator(ctx context.Context, operator domain.Operator) (result domain.Operator, err error) {
	err = s.run(ctx, s.tx.RunPlatform, "update_operator", func(txCtx context.Context, stats *domain.OperationStats) error {
		if operator.ID <= 0 {
			return domain.ErrOperatorNotFound
		}
		if validateErr := operator.Validate(); validateErr != nil {
			return validateErr
		}
		found, queryStats, updateErr := s.operators.UpdateOperator(txCtx, operator)
		stats.Add(queryStats)
		if updateErr != nil {
			return updateErr
		}
		if !found {
			return domain.ErrOperatorNotFound
		}
		result = operator
		return nil
	})
	return result, err
}

func (s *Service) DeleteOperator(ctx context.Context, id int64) error {
	return s.run(ctx, s.tx.RunPlatform, "delete_operator", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.operators.DeleteOperator(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) RecordOperatorLogin(ctx context.Context, id int64) error {
	return s.run(ctx, s.tx.RunPlatform, "record_operator_login", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.operators.RecordOperatorLogin(txCtx, id, time.Now())
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) IncrementOperatorMFAAttempts(ctx context.Context, id int64, threshold int, lockout time.Duration) (result domain.OperatorMFAAttempts, err error) {
	err = s.run(ctx, s.tx.RunPlatform, "increment_operator_mfa_attempts", func(txCtx context.Context, stats *domain.OperationStats) error {
		attempts, found, queryStats, incrementErr := s.operators.IncrementOperatorMFAAttempts(txCtx, id, threshold, time.Now().Add(lockout))
		stats.Add(queryStats)
		if incrementErr != nil {
			return incrementErr
		}
		if !found {
			return domain.ErrOperatorNotFound
		}
		result = attempts
		return nil
	})
	return result, err
}

func (s *Service) ResetOperatorMFAAttempts(ctx context.Context, id int64) error {
	return s.run(ctx, s.tx.RunPlatform, "reset_operator_mfa_attempts", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.operators.ResetOperatorMFAAttempts(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) FindOperatorSessionForUpdate(ctx context.Context, token string) (result domain.OperatorSession, err error) {
	err = s.run(ctx, s.tx.RunPlatform, "find_operator_session_for_update", func(txCtx context.Context, stats *domain.OperationStats) error {
		if token == "" {
			return domain.ErrOperatorSessionNotFound
		}
		session, found, queryStats, findErr := s.operators.FindOperatorSessionByToken(txCtx, token, true)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrOperatorSessionNotFound
		}
		result = session
		return nil
	})
	return result, err
}

func (s *Service) LatestOperatorSessionInFamily(ctx context.Context, familyID string) (result domain.OperatorSession, err error) {
	err = s.run(ctx, s.tx.RunPlatform, "latest_operator_session_in_family", func(txCtx context.Context, stats *domain.OperationStats) error {
		if familyID == "" {
			return domain.ErrOperatorSessionNotFound
		}
		session, found, queryStats, findErr := s.operators.LatestOperatorSessionInFamily(txCtx, familyID)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrOperatorSessionNotFound
		}
		result = session
		return nil
	})
	return result, err
}

func (s *Service) CreateOperatorSession(ctx context.Context, session domain.OperatorSession) (result domain.OperatorSession, err error) {
	err = s.run(ctx, s.tx.RunPlatform, "create_operator_session", func(txCtx context.Context, stats *domain.OperationStats) error {
		if validateErr := session.Validate(); validateErr != nil {
			return validateErr
		}
		stored, queryStats, insertErr := s.operators.InsertOperatorSession(txCtx, session)
		stats.Add(queryStats)
		if insertErr != nil {
			return insertErr
		}
		result = stored
		return nil
	})
	return result, err
}

// MarkOperatorSessionRotated records the hand-off exactly once: a session that
// was already rotated (or never existed) is reported, never silently
// re-rotated, because a second hand-off is the replay signal the refresh
// flow revokes the whole family on.
func (s *Service) MarkOperatorSessionRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) error {
	return s.run(ctx, s.tx.RunPlatform, "mark_operator_session_rotated", func(txCtx context.Context, stats *domain.OperationStats) error {
		rotated, queryStats, err := s.operators.MarkOperatorSessionRotated(txCtx, id, replacementToken, recoveryProofHash, rotatedAt)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !rotated {
			return domain.ErrOperatorSessionRotated
		}
		return nil
	})
}

func (s *Service) DeleteExpiredRotatedOperatorSessions(ctx context.Context, familyID string, now time.Time) error {
	return s.run(ctx, s.tx.RunPlatform, "delete_expired_rotated_operator_sessions", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.operators.DeleteExpiredRotatedOperatorSessions(txCtx, familyID, now)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) DeleteOperatorSession(ctx context.Context, id int64) error {
	return s.run(ctx, s.tx.RunPlatform, "delete_operator_session", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.operators.DeleteOperatorSession(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) RevokeOperatorSessions(ctx context.Context, operatorID int64) (result []domain.OperatorSession, err error) {
	err = s.run(ctx, s.tx.RunPlatform, "revoke_operator_sessions", func(txCtx context.Context, stats *domain.OperationStats) error {
		deleted, queryStats, deleteErr := s.operators.DeleteOperatorSessionsByOperator(txCtx, operatorID)
		stats.Add(queryStats)
		if deleteErr != nil {
			return deleteErr
		}
		result = deleted
		return nil
	})
	return result, err
}

func (s *Service) RevokeOperatorSessionFamily(ctx context.Context, familyID string) (result []domain.OperatorSession, err error) {
	err = s.run(ctx, s.tx.RunPlatform, "revoke_operator_session_family", func(txCtx context.Context, stats *domain.OperationStats) error {
		deleted, queryStats, deleteErr := s.operators.DeleteOperatorSessionsByFamily(txCtx, familyID)
		stats.Add(queryStats)
		if deleteErr != nil {
			return deleteErr
		}
		result = deleted
		return nil
	})
	return result, err
}

func (s *Service) DeleteExpiredOperatorSessions(ctx context.Context, now time.Time) (result int, err error) {
	err = s.run(ctx, s.tx.RunPlatform, "delete_expired_operator_sessions", func(txCtx context.Context, stats *domain.OperationStats) error {
		deleted, queryStats, deleteErr := s.operators.DeleteExpiredOperatorSessions(txCtx, now)
		stats.Add(queryStats)
		if deleteErr != nil {
			return deleteErr
		}
		result = deleted
		return nil
	})
	return result, err
}
