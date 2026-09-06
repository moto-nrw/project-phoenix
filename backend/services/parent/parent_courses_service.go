// Kurse in der Eltern-App (#3075, SH 4.3, ADR 0012).
//
// Diese Datei ist die Elternseite des Angebotswegs: sie liest den Kurskatalog
// und reicht Anfrage und Rücknahme an den Enrollment-Dienst weiter. Die
// Entscheidung trifft die OGS in der bestehenden Freigabeansicht.
package parent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	enrollmentSvc "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// GetChildCourses returns the school's courses with this child's state.
// Reading needs the same permission as asking: the catalog shows how full a
// course is, and that is not general child data.
func (s *service) GetChildCourses(
	ctx context.Context,
	accountID, studentID int64,
) (*enrollmentSvc.CourseCatalog, error) {
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionRequestSubmit)
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
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		resolved, resolveErr := s.OfferingChanges.CourseCatalog(txCtx, studentID, accountID)
		if resolveErr != nil {
			return resolveErr
		}
		catalog = resolved
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: get child courses: %w", txErr)
	}
	return catalog, nil
}

// RequestChildCourse asks the OGS for one course. Requires
// parent_portal.request.submit, the permission every parent request uses.
func (s *service) RequestChildCourse(
	ctx context.Context,
	accountID, studentID, offeringID int64,
	note string,
) (*enrollmentSvc.CourseCatalog, error) {
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionRequestSubmit)
	if err != nil {
		return nil, err
	}
	if s.OfferingChanges == nil {
		return nil, enrollmentSvc.ErrCourseRequestsDisabled
	}
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
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
		slog.Int64("tenant_id", child.tenantID),
	)
	return s.GetChildCourses(ctx, accountID, studentID)
}

// WithdrawChildCourseRequest takes back the caller's own open course request.
// It stays available after the school switches courses off, so an outstanding
// request can always be wound down.
func (s *service) WithdrawChildCourseRequest(
	ctx context.Context,
	accountID, studentID, requestID int64,
) (*enrollmentSvc.CourseCatalog, error) {
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	if s.OfferingChanges == nil {
		return nil, enrollmentSvc.ErrCourseRequestsDisabled
	}
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return s.OfferingChanges.WithdrawCourseRequest(txCtx, requestID, accountID, studentID)
	})
	if txErr != nil {
		return nil, txErr
	}
	return s.GetChildCourses(ctx, accountID, studentID)
}
