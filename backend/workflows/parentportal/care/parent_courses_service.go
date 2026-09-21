// Kurse in der Eltern-App (#3075, SH 4.3, ADR 0012).
//
// Diese Datei ist die Elternseite des Angebotswegs: sie liest den Kurskatalog
// und reicht Anfrage und Rücknahme an den Enrollment-Dienst weiter. Die
// Entscheidung trifft die OGS in der bestehenden Freigabeansicht.
package care

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	enrollmentSvc "github.com/moto-nrw/project-phoenix/services/enrollment"
)

// GetChildCourses returns the school's courses with this child's state.
// Reading requires parent_portal.enrollments.view; whether the guardian may
// also ask is reported separately through CanRequest.
func (s *Service) GetChildCourses(
	ctx context.Context,
	accountID, studentID int64,
) (*enrollmentSvc.CourseCatalog, error) {
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionEnrollmentsView)
	if err != nil {
		// "Darf nicht anfragen" ist kein Fehler, sondern ein benannter Grund:
		// sonst stünde an der Stelle eine Fehlermeldung statt einer Erklärung.
		if errors.Is(err, ErrGuardianPermissionDenied) {
			return &enrollmentSvc.CourseCatalog{
				DisabledReason: enrollmentSvc.CourseRequestsReasonNoPermission,
				Items:          []enrollmentSvc.CourseCatalogItem{},
			}, nil
		}
		return nil, err
	}
	if s.OfferingChanges == nil {
		return &enrollmentSvc.CourseCatalog{
			DisabledReason: enrollmentSvc.CourseRequestsReasonSchoolOff,
			Items:          []enrollmentSvc.CourseCatalogItem{},
		}, nil
	}
	var catalog *enrollmentSvc.CourseCatalog
	txErr := InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		resolved, resolveErr := s.OfferingChanges.CourseCatalog(txCtx, studentID, accountID)
		if resolveErr != nil {
			return resolveErr
		}
		catalog = resolved
		catalog.CanRequest = child.HasPermission(authorize.GuardianPermissionEnrollmentSubmit)
		catalog.ReasonRequired = s.GuardianReasonRequired(txCtx, child.TenantID)
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: get child courses: %w", txErr)
	}
	return catalog, nil
}

// RequestChildCourse asks the OGS for one course. The response is the course
// catalog, so its view permission is checked with the submit permission before
// the request is written.
func (s *Service) RequestChildCourse(
	ctx context.Context,
	accountID, studentID, offeringID int64,
	note string,
) (*enrollmentSvc.CourseCatalog, error) {
	child, err := s.resolveCourseWriteChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	if err := child.RequireCareRunning(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(note) == "" && s.GuardianReasonRequired(ctx, child.TenantID) {
		return nil, ErrEmptyNote
	}
	if s.OfferingChanges == nil {
		return nil, enrollmentSvc.ErrCourseRequestsDisabled
	}
	txErr := InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		if err := s.RequireCareRunningForUpdate(txCtx, studentID); err != nil {
			return err
		}
		_, createErr := s.OfferingChanges.CreateCourseRequest(txCtx, enrollmentSvc.CreateCourseRequestInput{
			StudentID:  studentID,
			AccountID:  accountID,
			OfferingID: offeringID,
			Note:       note,
		})
		return createErr
	})
	if txErr != nil {
		return nil, txErr
	}
	s.Logger.Info("parent requested course",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("care_offering_id", offeringID),
		slog.Int64("tenant_id", child.TenantID),
	)
	return s.GetChildCourses(ctx, accountID, studentID)
}

// WithdrawChildCourseRequest takes back the caller's own open course request
// while course requests are enabled. When the school switches the feature off,
// the section and its actions are intentionally hidden; the OGS can still
// decide the outstanding request in the existing review queue.
func (s *Service) WithdrawChildCourseRequest(
	ctx context.Context,
	accountID, studentID, requestID int64,
) (*enrollmentSvc.CourseCatalog, error) {
	child, err := s.resolveCourseWriteChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	if err := child.RequireCareRunning(); err != nil {
		return nil, err
	}
	if s.OfferingChanges == nil {
		return nil, enrollmentSvc.ErrCourseRequestsDisabled
	}
	txErr := InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		if err := s.RequireCareRunningForUpdate(txCtx, studentID); err != nil {
			return err
		}
		return s.OfferingChanges.WithdrawCourseRequest(txCtx, requestID, accountID, studentID)
	})
	if txErr != nil {
		return nil, txErr
	}
	return s.GetChildCourses(ctx, accountID, studentID)
}

// resolveCourseWriteChild checks both permissions needed by a course write:
// submit authorizes the mutation and enrollments.view authorizes the catalog
// returned after it. Checking both before the tenant transaction prevents a
// successful write from being reported as a permission error.
func (s *Service) resolveCourseWriteChild(
	ctx context.Context,
	accountID, studentID int64,
) (*Child, error) {
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionEnrollmentSubmit)
	if err != nil {
		return nil, err
	}
	if !child.HasPermission(authorize.GuardianPermissionEnrollmentsView) {
		return nil, ErrGuardianPermissionDenied
	}
	return child, nil
}
