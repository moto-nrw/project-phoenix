package fixed

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"github.com/moto-nrw/project-phoenix/models/platform"
)

// seedOperators creates platform operator accounts for development
func (s *Seeder) seedOperators(ctx context.Context) error {
	operators := []struct {
		email       string
		displayName string
		password    string
	}{
		{"operator@example.com", "Administrator", "Test1234%"},
	}

	for _, data := range operators {
		passwordHash, err := userpass.HashPassword(data.password, nil)
		if err != nil {
			return fmt.Errorf("failed to hash password for operator %s: %w", data.email, err)
		}

		operator := &platform.Operator{
			Email:        data.email,
			DisplayName:  data.displayName,
			PasswordHash: passwordHash,
			Active:       true,
		}
		operator.CreatedAt = time.Now()
		operator.UpdatedAt = time.Now()

		_, err = s.tx.NewInsert().Model(operator).
			ModelTableExpr("platform.operators").
			On("CONFLICT (email) DO UPDATE").
			Set("display_name = EXCLUDED.display_name").
			Set(SQLExcludedUpdatedAt).
			Returning(SQLBaseColumns).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to upsert operator %s: %w", data.email, err)
		}

		s.result.Operators = append(s.result.Operators, operator)
	}

	if s.verbose {
		log.Printf("Created %d operator(s)", len(s.result.Operators))
	}

	return nil
}
