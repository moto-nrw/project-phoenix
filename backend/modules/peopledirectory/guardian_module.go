package peopledirectory

import (
	"context"
	"strings"
)

// The guardian methods of the module validate the input the facade can
// judge on its own, then delegate: the row-level reads to the engine the
// composition built, everything else to the bound provider. Each
// provider-backed call is observed under a stable operation name.

// guardianQuery runs one provider-backed read.
func guardianQuery[T any](m *Module, operation string, fn func(GuardianProvider) (T, error)) (T, error) {
	var result T
	err := m.observed(operation, func() error {
		provider, err := m.guardians.Load()
		if err != nil {
			return err
		}
		result, err = fn(provider)
		return err
	})
	return result, err
}

// guardianCommand runs one provider-backed write.
func guardianCommand(m *Module, operation string, fn func(GuardianProvider) error) error {
	return m.observed(operation, func() error {
		provider, err := m.guardians.Load()
		if err != nil {
			return err
		}
		return fn(provider)
	})
}

func (m *Module) FindGuardian(ctx context.Context, id int64) (Guardian, error) {
	if id <= 0 {
		return Guardian{}, invalidGuardian("guardian ID is required")
	}
	return guardianQuery(m, "find_guardian", func(p GuardianProvider) (Guardian, error) { return p.FindGuardian(ctx, id) })
}

func (m *Module) ListGuardians(ctx context.Context, page, pageSize int) ([]Guardian, error) {
	if page < 0 || pageSize < 0 {
		return nil, invalidGuardian("page and page size must not be negative")
	}
	return guardianQuery(m, "list_guardians", func(p GuardianProvider) ([]Guardian, error) { return p.ListGuardians(ctx, page, pageSize) })
}

func (m *Module) ListGuardiansWithoutAccount(ctx context.Context) ([]Guardian, error) {
	return guardianQuery(m, "list_guardians_without_account", func(p GuardianProvider) ([]Guardian, error) {
		return p.ListGuardiansWithoutAccount(ctx)
	})
}

func (m *Module) ListInvitableGuardians(ctx context.Context) ([]Guardian, error) {
	return guardianQuery(m, "list_invitable_guardians", func(p GuardianProvider) ([]Guardian, error) {
		return p.ListInvitableGuardians(ctx)
	})
}

func (m *Module) SearchGuardians(ctx context.Context, text string, limit int) ([]GuardianMatch, error) {
	text = strings.TrimSpace(text)
	if text == "" || limit <= 0 {
		return []GuardianMatch{}, nil
	}
	return guardianQuery(m, "search_guardians", func(p GuardianProvider) ([]GuardianMatch, error) {
		return p.SearchGuardians(ctx, text, limit)
	})
}

func (m *Module) GuardianDeleteImpact(ctx context.Context, id int64) (GuardianDeleteImpact, error) {
	if id <= 0 {
		return GuardianDeleteImpact{}, invalidGuardian("guardian ID is required")
	}
	return guardianQuery(m, "guardian_delete_impact", func(p GuardianProvider) (GuardianDeleteImpact, error) {
		return p.GuardianDeleteImpact(ctx, id)
	})
}

func (m *Module) ListGuardianPhones(ctx context.Context, guardianID int64) ([]GuardianPhone, error) {
	if guardianID <= 0 {
		return nil, invalidGuardian("guardian ID is required")
	}
	return guardianQuery(m, "list_guardian_phones", func(p GuardianProvider) ([]GuardianPhone, error) {
		return p.ListGuardianPhones(ctx, guardianID)
	})
}

func (m *Module) FindGuardianPhone(ctx context.Context, phoneID int64) (GuardianPhone, error) {
	if phoneID <= 0 {
		return GuardianPhone{}, invalidGuardian("phone ID is required")
	}
	return guardianQuery(m, "find_guardian_phone", func(p GuardianProvider) (GuardianPhone, error) {
		return p.FindGuardianPhone(ctx, phoneID)
	})
}

func (m *Module) ListStudentGuardians(ctx context.Context, studentID int64) ([]GuardianWithLink, error) {
	if studentID <= 0 {
		return nil, invalidGuardian("student ID is required")
	}
	return guardianQuery(m, "list_student_guardians", func(p GuardianProvider) ([]GuardianWithLink, error) {
		return p.ListStudentGuardians(ctx, studentID)
	})
}

func (m *Module) ListGuardianStudents(ctx context.Context, guardianID int64) ([]StudentWithLink, error) {
	if guardianID <= 0 {
		return nil, invalidGuardian("guardian ID is required")
	}
	return guardianQuery(m, "list_guardian_students", func(p GuardianProvider) ([]StudentWithLink, error) {
		return p.ListGuardianStudents(ctx, guardianID)
	})
}

func (m *Module) FindGuardianLink(ctx context.Context, linkID int64) (GuardianLink, error) {
	if linkID <= 0 {
		return GuardianLink{}, invalidGuardian("relationship ID is required")
	}
	return guardianQuery(m, "find_guardian_link", func(p GuardianProvider) (GuardianLink, error) {
		return p.FindGuardianLink(ctx, linkID)
	})
}

func (m *Module) ListGuardianLinksByAccount(ctx context.Context, accountID int64) ([]GuardianLink, error) {
	if accountID <= 0 {
		return nil, invalidGuardian("account ID is required")
	}
	return m.engine.ListGuardianLinksByAccount(ctx, accountID)
}

func (m *Module) ListGuardiansByAccount(ctx context.Context, accountIDs []int64) ([]Guardian, error) {
	accountIDs = uniquePositive(accountIDs)
	if len(accountIDs) == 0 {
		return []Guardian{}, nil
	}
	return m.engine.ListGuardiansByAccounts(ctx, accountIDs)
}

func (m *Module) ListGuardiansByID(ctx context.Context, ids []int64) ([]Guardian, error) {
	ids = uniquePositive(ids)
	if len(ids) == 0 {
		return []Guardian{}, nil
	}
	return m.engine.ListGuardiansByIDs(ctx, ids)
}

func (m *Module) CountGuardianLinks(ctx context.Context, guardianIDs []int64) (map[int64]int, error) {
	guardianIDs = uniquePositive(guardianIDs)
	if len(guardianIDs) == 0 {
		return map[int64]int{}, nil
	}
	return m.engine.CountGuardianLinks(ctx, guardianIDs)
}

func (m *Module) GuardianPaymentMasked(ctx context.Context, guardianID int64, actor GuardianPaymentActor) (GuardianPayment, error) {
	if guardianID <= 0 {
		return GuardianPayment{}, invalidGuardian("guardian ID is required")
	}
	if actor.AccountID <= 0 {
		return GuardianPayment{}, invalidGuardian("acting account is required")
	}
	return guardianQuery(m, "guardian_payment_masked", func(p GuardianProvider) (GuardianPayment, error) {
		return p.GuardianPaymentMasked(ctx, guardianID, actor)
	})
}

func (m *Module) ListPaymentOverview(ctx context.Context, actor GuardianPaymentActor) ([]GuardianPaymentRow, error) {
	if actor.AccountID <= 0 {
		return nil, invalidGuardian("acting account is required")
	}
	return guardianQuery(m, "list_payment_overview", func(p GuardianProvider) ([]GuardianPaymentRow, error) {
		return p.ListPaymentOverview(ctx, actor)
	})
}

func (m *Module) ListPaymentExportRows(ctx context.Context, actor GuardianPaymentActor, format string) ([]GuardianPaymentRow, error) {
	if actor.AccountID <= 0 {
		return nil, invalidGuardian("acting account is required")
	}
	return guardianQuery(m, "list_payment_export_rows", func(p GuardianProvider) ([]GuardianPaymentRow, error) {
		return p.ListPaymentExportRows(ctx, actor, format)
	})
}

func (m *Module) CreateGuardian(ctx context.Context, input GuardianInput) (Guardian, error) {
	return guardianQuery(m, "create_guardian", func(p GuardianProvider) (Guardian, error) { return p.CreateGuardian(ctx, input) })
}

func (m *Module) UpdateGuardian(ctx context.Context, id int64, input GuardianInput) error {
	if id <= 0 {
		return invalidGuardian("guardian ID is required")
	}
	return guardianCommand(m, "update_guardian", func(p GuardianProvider) error { return p.UpdateGuardian(ctx, id, input) })
}

func (m *Module) EvaluateGuardianDelete(ctx context.Context, id int64, force, isAdmin bool) (bool, error) {
	if id <= 0 {
		return false, invalidGuardian("guardian ID is required")
	}
	return guardianQuery(m, "evaluate_guardian_delete", func(p GuardianProvider) (bool, error) {
		return p.EvaluateGuardianDelete(ctx, id, force, isAdmin)
	})
}

func (m *Module) DeleteGuardian(ctx context.Context, input GuardianDelete) error {
	if input.GuardianID <= 0 {
		return invalidGuardian("guardian ID is required")
	}
	if input.ActorAccountID <= 0 {
		return invalidGuardian("acting account is required")
	}
	return guardianCommand(m, "delete_guardian", func(p GuardianProvider) error { return p.DeleteGuardian(ctx, input) })
}

func (m *Module) AddGuardianPhone(ctx context.Context, guardianID int64, input GuardianPhoneInput) (GuardianPhone, error) {
	if guardianID <= 0 {
		return GuardianPhone{}, invalidGuardian("guardian ID is required")
	}
	if strings.TrimSpace(input.PhoneNumber) == "" {
		return GuardianPhone{}, invalidGuardian("phone_number is required")
	}
	return guardianQuery(m, "add_guardian_phone", func(p GuardianProvider) (GuardianPhone, error) {
		return p.AddGuardianPhone(ctx, guardianID, input)
	})
}

func (m *Module) UpdateGuardianPhone(ctx context.Context, phoneID int64, input GuardianPhoneUpdate) error {
	if phoneID <= 0 {
		return invalidGuardian("phone ID is required")
	}
	return guardianCommand(m, "update_guardian_phone", func(p GuardianProvider) error { return p.UpdateGuardianPhone(ctx, phoneID, input) })
}

func (m *Module) DeleteGuardianPhone(ctx context.Context, phoneID int64) error {
	if phoneID <= 0 {
		return invalidGuardian("phone ID is required")
	}
	return guardianCommand(m, "delete_guardian_phone", func(p GuardianProvider) error { return p.DeleteGuardianPhone(ctx, phoneID) })
}

func (m *Module) SetPrimaryGuardianPhone(ctx context.Context, phoneID int64) error {
	if phoneID <= 0 {
		return invalidGuardian("phone ID is required")
	}
	return guardianCommand(m, "set_primary_guardian_phone", func(p GuardianProvider) error { return p.SetPrimaryGuardianPhone(ctx, phoneID) })
}

func (m *Module) LinkGuardianToStudent(ctx context.Context, input LinkGuardian) (GuardianLink, error) {
	if input.StudentID <= 0 || input.GuardianProfileID <= 0 {
		return GuardianLink{}, invalidGuardian("student ID and guardian ID are required")
	}
	if strings.TrimSpace(input.RelationshipType) == "" {
		return GuardianLink{}, invalidGuardian("relationship_type is required")
	}
	return guardianQuery(m, "link_guardian_to_student", func(p GuardianProvider) (GuardianLink, error) {
		return p.LinkGuardianToStudent(ctx, input)
	})
}

func (m *Module) ValidateNewGuardians(ctx context.Context, guardians []NewStudentGuardian) error {
	return guardianCommand(m, "validate_new_guardians", func(p GuardianProvider) error { return p.ValidateNewGuardians(ctx, guardians) })
}

func (m *Module) AddGuardiansToStudent(ctx context.Context, studentID int64, guardians []NewStudentGuardian) error {
	if studentID <= 0 {
		return invalidGuardian("student ID is required")
	}
	return guardianCommand(m, "add_guardians_to_student", func(p GuardianProvider) error {
		return p.AddGuardiansToStudent(ctx, studentID, guardians)
	})
}

func (m *Module) UpdateGuardianLink(ctx context.Context, linkID int64, input GuardianLinkUpdate) error {
	if linkID <= 0 {
		return invalidGuardian("relationship ID is required")
	}
	return guardianCommand(m, "update_guardian_link", func(p GuardianProvider) error { return p.UpdateGuardianLink(ctx, linkID, input) })
}

func (m *Module) RemoveGuardianFromStudent(ctx context.Context, input RemoveGuardian) error {
	if input.StudentID <= 0 || input.GuardianProfileID <= 0 {
		return invalidGuardian("student ID and guardian ID are required")
	}
	if input.ActorAccountID <= 0 {
		return invalidGuardian("acting account is required")
	}
	return guardianCommand(m, "remove_guardian_from_student", func(p GuardianProvider) error { return p.RemoveGuardianFromStudent(ctx, input) })
}

func (m *Module) RevealGuardianPayment(ctx context.Context, guardianID int64, actor GuardianPaymentActor) (GuardianPayment, error) {
	if guardianID <= 0 {
		return GuardianPayment{}, invalidGuardian("guardian ID is required")
	}
	if actor.AccountID <= 0 {
		return GuardianPayment{}, invalidGuardian("acting account is required")
	}
	return guardianQuery(m, "reveal_guardian_payment", func(p GuardianProvider) (GuardianPayment, error) {
		return p.RevealGuardianPayment(ctx, guardianID, actor)
	})
}

func (m *Module) UpdateGuardianPayment(ctx context.Context, guardianID int64, input GuardianPaymentInput) error {
	if guardianID <= 0 {
		return invalidGuardian("guardian ID is required")
	}
	if input.ActorAccountID <= 0 {
		return invalidGuardian("acting account is required")
	}
	return guardianCommand(m, "update_guardian_payment", func(p GuardianProvider) error { return p.UpdateGuardianPayment(ctx, guardianID, input) })
}

func (m *Module) SetStudentPayer(ctx context.Context, input StudentPayer) error {
	if input.StudentID <= 0 {
		return ErrGuardianStudentRequired
	}
	if input.ActorAccountID <= 0 {
		return invalidGuardian("acting account is required")
	}
	return guardianCommand(m, "set_student_payer", func(p GuardianProvider) error { return p.SetStudentPayer(ctx, input) })
}
