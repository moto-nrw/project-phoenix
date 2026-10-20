package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

func (s *Service) FindGuardian(ctx context.Context, id int64) (peopledirectory.Guardian, error) {
	if id <= 0 {
		return peopledirectory.Guardian{}, invalidGuardian("guardian ID is required")
	}
	return guardianQuery(s, "find_guardian", func(p peopledirectory.GuardianProvider) (peopledirectory.Guardian, error) {
		return p.FindGuardian(ctx, id)
	})
}

func (s *Service) ListGuardians(ctx context.Context, page, pageSize int) ([]peopledirectory.Guardian, error) {
	if page < 0 || pageSize < 0 {
		return nil, invalidGuardian("page and page size must not be negative")
	}
	return guardianQuery(s, "list_guardians", func(p peopledirectory.GuardianProvider) ([]peopledirectory.Guardian, error) {
		return p.ListGuardians(ctx, page, pageSize)
	})
}

func (s *Service) ListGuardiansWithoutAccount(ctx context.Context) ([]peopledirectory.Guardian, error) {
	return guardianQuery(s, "list_guardians_without_account", func(p peopledirectory.GuardianProvider) ([]peopledirectory.Guardian, error) {
		return p.ListGuardiansWithoutAccount(ctx)
	})
}

func (s *Service) ListInvitableGuardians(ctx context.Context) ([]peopledirectory.Guardian, error) {
	return guardianQuery(s, "list_invitable_guardians", func(p peopledirectory.GuardianProvider) ([]peopledirectory.Guardian, error) {
		return p.ListInvitableGuardians(ctx)
	})
}

func (s *Service) SearchGuardians(ctx context.Context, text string, limit int) ([]peopledirectory.GuardianMatch, error) {
	text = strings.TrimSpace(text)
	if text == "" || limit <= 0 {
		return []peopledirectory.GuardianMatch{}, nil
	}
	return guardianQuery(s, "search_guardians", func(p peopledirectory.GuardianProvider) ([]peopledirectory.GuardianMatch, error) {
		return p.SearchGuardians(ctx, text, limit)
	})
}

func (s *Service) FindGuardianByEmail(ctx context.Context, email string) (peopledirectory.Guardian, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return peopledirectory.Guardian{}, invalidGuardian("guardian e-mail is required")
	}
	return guardianQuery(s, "find_guardian_by_email", func(p peopledirectory.GuardianProvider) (peopledirectory.Guardian, error) {
		return p.FindGuardianByEmail(ctx, email)
	})
}

func (s *Service) GuardianDeleteImpact(ctx context.Context, id int64) (peopledirectory.GuardianDeleteImpact, error) {
	if id <= 0 {
		return peopledirectory.GuardianDeleteImpact{}, invalidGuardian("guardian ID is required")
	}
	return guardianQuery(s, "guardian_delete_impact", func(p peopledirectory.GuardianProvider) (peopledirectory.GuardianDeleteImpact, error) {
		return p.GuardianDeleteImpact(ctx, id)
	})
}

func (s *Service) ListGuardianPhones(ctx context.Context, guardianID int64) ([]peopledirectory.GuardianPhone, error) {
	if guardianID <= 0 {
		return nil, invalidGuardian("guardian ID is required")
	}
	return guardianQuery(s, "list_guardian_phones", func(p peopledirectory.GuardianProvider) ([]peopledirectory.GuardianPhone, error) {
		return p.ListGuardianPhones(ctx, guardianID)
	})
}

func (s *Service) FindGuardianPhone(ctx context.Context, phoneID int64) (peopledirectory.GuardianPhone, error) {
	if phoneID <= 0 {
		return peopledirectory.GuardianPhone{}, invalidGuardian("phone ID is required")
	}
	return guardianQuery(s, "find_guardian_phone", func(p peopledirectory.GuardianProvider) (peopledirectory.GuardianPhone, error) {
		return p.FindGuardianPhone(ctx, phoneID)
	})
}

func (s *Service) ListStudentGuardians(ctx context.Context, studentID int64) ([]peopledirectory.GuardianWithLink, error) {
	if studentID <= 0 {
		return nil, invalidGuardian("student ID is required")
	}
	return guardianQuery(s, "list_student_guardians", func(p peopledirectory.GuardianProvider) ([]peopledirectory.GuardianWithLink, error) {
		return p.ListStudentGuardians(ctx, studentID)
	})
}

func (s *Service) ListGuardianStudents(ctx context.Context, guardianID int64) ([]peopledirectory.StudentWithLink, error) {
	if guardianID <= 0 {
		return nil, invalidGuardian("guardian ID is required")
	}
	return guardianQuery(s, "list_guardian_students", func(p peopledirectory.GuardianProvider) ([]peopledirectory.StudentWithLink, error) {
		return p.ListGuardianStudents(ctx, guardianID)
	})
}

func (s *Service) FindGuardianLink(ctx context.Context, linkID int64) (peopledirectory.GuardianLink, error) {
	if linkID <= 0 {
		return peopledirectory.GuardianLink{}, invalidGuardian("relationship ID is required")
	}
	return guardianQuery(s, "find_guardian_link", func(p peopledirectory.GuardianProvider) (peopledirectory.GuardianLink, error) {
		return p.FindGuardianLink(ctx, linkID)
	})
}

func (s *Service) ListGuardianLinksByAccount(ctx context.Context, accountID int64) ([]peopledirectory.GuardianLink, error) {
	if accountID <= 0 {
		return nil, invalidGuardian("account ID is required")
	}

	values, err := s.guardianRows.ListLinksByAccount(ctx, accountID)
	return nonNilGuardianLinks(values), MapError(err)
}

func (s *Service) ListGuardiansByAccount(ctx context.Context, accountIDs []int64) ([]peopledirectory.Guardian, error) {
	accountIDs = peopledirectory.PositiveIDs(accountIDs)
	if len(accountIDs) == 0 {
		return []peopledirectory.Guardian{}, nil
	}

	values, err := s.guardianRows.ListByAccounts(ctx, accountIDs)
	return toPublicGuardians(values), MapError(err)
}

func (s *Service) ListGuardiansByID(ctx context.Context, ids []int64) ([]peopledirectory.Guardian, error) {
	ids = peopledirectory.PositiveIDs(ids)
	if len(ids) == 0 {
		return []peopledirectory.Guardian{}, nil
	}

	values, err := s.guardianRows.ListByIDs(ctx, ids)
	return toPublicGuardians(values), MapError(err)
}

func (s *Service) CountGuardianLinks(ctx context.Context, guardianIDs []int64) (map[int64]int, error) {
	guardianIDs = peopledirectory.PositiveIDs(guardianIDs)
	if len(guardianIDs) == 0 {
		return map[int64]int{}, nil
	}

	values, err := s.guardianRows.CountLinks(ctx, guardianIDs)
	if values == nil {
		values = map[int64]int{}
	}
	return values, MapError(err)
}

func (s *Service) StudentsWithPortalGuardian(ctx context.Context, studentIDs []int64) (map[int64]bool, error) {
	studentIDs = peopledirectory.PositiveIDs(studentIDs)
	if len(studentIDs) == 0 {
		return map[int64]bool{}, nil
	}

	values, err := s.guardianRows.StudentsWithPortalGuardian(ctx, studentIDs)
	if values == nil {
		values = map[int64]bool{}
	}
	return values, MapError(err)
}

func (s *Service) GuardianPaymentMasked(ctx context.Context, guardianID int64, actor peopledirectory.GuardianPaymentActor) (peopledirectory.GuardianPayment, error) {
	if guardianID <= 0 {
		return peopledirectory.GuardianPayment{}, invalidGuardian("guardian ID is required")
	}
	if actor.AccountID <= 0 {
		return peopledirectory.GuardianPayment{}, invalidGuardian("acting account is required")
	}
	return guardianQuery(s, "guardian_payment_masked", func(p peopledirectory.GuardianProvider) (peopledirectory.GuardianPayment, error) {
		return p.GuardianPaymentMasked(ctx, guardianID, actor)
	})
}

func (s *Service) ListPaymentOverview(ctx context.Context, actor peopledirectory.GuardianPaymentActor) ([]peopledirectory.GuardianPaymentRow, error) {
	if actor.AccountID <= 0 {
		return nil, invalidGuardian("acting account is required")
	}
	return guardianQuery(s, "list_payment_overview", func(p peopledirectory.GuardianProvider) ([]peopledirectory.GuardianPaymentRow, error) {
		return p.ListPaymentOverview(ctx, actor)
	})
}

func (s *Service) ListPaymentExportRows(ctx context.Context, actor peopledirectory.GuardianPaymentActor, format string) ([]peopledirectory.GuardianPaymentRow, error) {
	if actor.AccountID <= 0 {
		return nil, invalidGuardian("acting account is required")
	}
	return guardianQuery(s, "list_payment_export_rows", func(p peopledirectory.GuardianProvider) ([]peopledirectory.GuardianPaymentRow, error) {
		return p.ListPaymentExportRows(ctx, actor, format)
	})
}

func (s *Service) CreateGuardian(ctx context.Context, input peopledirectory.GuardianInput) (peopledirectory.Guardian, error) {
	return guardianQuery(s, "create_guardian", func(p peopledirectory.GuardianProvider) (peopledirectory.Guardian, error) {
		return p.CreateGuardian(ctx, input)
	})
}

func (s *Service) UpdateGuardian(ctx context.Context, id int64, input peopledirectory.GuardianInput) error {
	if id <= 0 {
		return invalidGuardian("guardian ID is required")
	}
	return guardianCommand(s, "update_guardian", func(p peopledirectory.GuardianProvider) error { return p.UpdateGuardian(ctx, id, input) })
}

func (s *Service) EvaluateGuardianDelete(ctx context.Context, id int64, force, isAdmin bool) (bool, error) {
	if id <= 0 {
		return false, invalidGuardian("guardian ID is required")
	}
	return guardianQuery(s, "evaluate_guardian_delete", func(p peopledirectory.GuardianProvider) (bool, error) {
		return p.EvaluateGuardianDelete(ctx, id, force, isAdmin)
	})
}

func (s *Service) DeleteGuardian(ctx context.Context, input peopledirectory.GuardianDelete) error {
	if input.GuardianID <= 0 {
		return invalidGuardian("guardian ID is required")
	}
	if input.ActorAccountID <= 0 {
		return invalidGuardian("acting account is required")
	}
	return guardianCommand(s, "delete_guardian", func(p peopledirectory.GuardianProvider) error { return p.DeleteGuardian(ctx, input) })
}

func (s *Service) AddGuardianPhone(ctx context.Context, guardianID int64, input peopledirectory.GuardianPhoneInput) (peopledirectory.GuardianPhone, error) {
	if guardianID <= 0 {
		return peopledirectory.GuardianPhone{}, invalidGuardian("guardian ID is required")
	}
	if strings.TrimSpace(input.PhoneNumber) == "" {
		return peopledirectory.GuardianPhone{}, invalidGuardian("Telefonnummer ist erforderlich")
	}
	return guardianQuery(s, "add_guardian_phone", func(p peopledirectory.GuardianProvider) (peopledirectory.GuardianPhone, error) {
		return p.AddGuardianPhone(ctx, guardianID, input)
	})
}

func (s *Service) UpdateGuardianPhone(ctx context.Context, phoneID int64, input peopledirectory.GuardianPhoneUpdate) error {
	if phoneID <= 0 {
		return invalidGuardian("phone ID is required")
	}
	return guardianCommand(s, "update_guardian_phone", func(p peopledirectory.GuardianProvider) error { return p.UpdateGuardianPhone(ctx, phoneID, input) })
}

func (s *Service) DeleteGuardianPhone(ctx context.Context, phoneID int64) error {
	if phoneID <= 0 {
		return invalidGuardian("phone ID is required")
	}
	return guardianCommand(s, "delete_guardian_phone", func(p peopledirectory.GuardianProvider) error { return p.DeleteGuardianPhone(ctx, phoneID) })
}

func (s *Service) SetPrimaryGuardianPhone(ctx context.Context, phoneID int64) error {
	if phoneID <= 0 {
		return invalidGuardian("phone ID is required")
	}
	return guardianCommand(s, "set_primary_guardian_phone", func(p peopledirectory.GuardianProvider) error { return p.SetPrimaryGuardianPhone(ctx, phoneID) })
}

func (s *Service) LinkGuardianToStudent(ctx context.Context, input peopledirectory.LinkGuardian) (peopledirectory.GuardianLink, error) {
	if input.StudentID <= 0 || input.GuardianProfileID <= 0 {
		return peopledirectory.GuardianLink{}, invalidGuardian("student ID and guardian ID are required")
	}
	if strings.TrimSpace(input.RelationshipType) == "" {
		return peopledirectory.GuardianLink{}, invalidGuardian("relationship_type is required")
	}
	return guardianQuery(s, "link_guardian_to_student", func(p peopledirectory.GuardianProvider) (peopledirectory.GuardianLink, error) {
		return p.LinkGuardianToStudent(ctx, input)
	})
}

func (s *Service) ValidateNewGuardians(ctx context.Context, guardians []peopledirectory.NewStudentGuardian) error {
	return guardianCommand(s, "validate_new_guardians", func(p peopledirectory.GuardianProvider) error { return p.ValidateNewGuardians(ctx, guardians) })
}

func (s *Service) AddGuardiansToStudent(ctx context.Context, studentID int64, guardians []peopledirectory.NewStudentGuardian) error {
	if studentID <= 0 {
		return invalidGuardian("student ID is required")
	}
	return guardianCommand(s, "add_guardians_to_student", func(p peopledirectory.GuardianProvider) error {
		return p.AddGuardiansToStudent(ctx, studentID, guardians)
	})
}

func (s *Service) UpdateGuardianLink(ctx context.Context, linkID int64, input peopledirectory.GuardianLinkUpdate) error {
	if linkID <= 0 {
		return invalidGuardian("relationship ID is required")
	}
	return guardianCommand(s, "update_guardian_link", func(p peopledirectory.GuardianProvider) error { return p.UpdateGuardianLink(ctx, linkID, input) })
}

func (s *Service) RemoveGuardianFromStudent(ctx context.Context, input peopledirectory.RemoveGuardian) error {
	if input.StudentID <= 0 || input.GuardianProfileID <= 0 {
		return invalidGuardian("student ID and guardian ID are required")
	}
	if input.ActorAccountID <= 0 {
		return invalidGuardian("acting account is required")
	}
	return guardianCommand(s, "remove_guardian_from_student", func(p peopledirectory.GuardianProvider) error { return p.RemoveGuardianFromStudent(ctx, input) })
}

func (s *Service) RevealGuardianPayment(ctx context.Context, guardianID int64, actor peopledirectory.GuardianPaymentActor) (peopledirectory.GuardianPayment, error) {
	if guardianID <= 0 {
		return peopledirectory.GuardianPayment{}, invalidGuardian("guardian ID is required")
	}
	if actor.AccountID <= 0 {
		return peopledirectory.GuardianPayment{}, invalidGuardian("acting account is required")
	}
	return guardianQuery(s, "reveal_guardian_payment", func(p peopledirectory.GuardianProvider) (peopledirectory.GuardianPayment, error) {
		return p.RevealGuardianPayment(ctx, guardianID, actor)
	})
}

func (s *Service) UpdateGuardianPayment(ctx context.Context, guardianID int64, input peopledirectory.GuardianPaymentInput) error {
	if guardianID <= 0 {
		return invalidGuardian("guardian ID is required")
	}
	if input.ActorAccountID <= 0 {
		return invalidGuardian("acting account is required")
	}
	return guardianCommand(s, "update_guardian_payment", func(p peopledirectory.GuardianProvider) error { return p.UpdateGuardianPayment(ctx, guardianID, input) })
}

func (s *Service) SetStudentPayer(ctx context.Context, input peopledirectory.StudentPayer) error {
	if input.StudentID <= 0 {
		return peopledirectory.ErrGuardianStudentRequired
	}
	if input.ActorAccountID <= 0 {
		return invalidGuardian("acting account is required")
	}
	return guardianCommand(s, "set_student_payer", func(p peopledirectory.GuardianProvider) error { return p.SetStudentPayer(ctx, input) })
}

func guardianQuery[T any](s *Service, operation string, fn func(peopledirectory.GuardianProvider) (T, error)) (T, error) {
	var result T
	err := s.observed(operation, func() error {
		provider, err := s.guardianProvider()
		if err != nil {
			return err
		}
		result, err = fn(provider)
		return err
	})
	return result, err
}

func guardianCommand(s *Service, operation string, fn func(peopledirectory.GuardianProvider) error) error {
	return s.observed(operation, func() error {
		provider, err := s.guardianProvider()
		if err != nil {
			return err
		}
		return fn(provider)
	})
}
