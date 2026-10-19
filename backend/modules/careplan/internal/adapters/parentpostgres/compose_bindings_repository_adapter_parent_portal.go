package parent

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

type ParentStudentDirectory struct {
	Directory careplan.ParentPortalDirectory
}

func (d ParentStudentDirectory) ListStudentsByID(ctx context.Context, ids []int64) ([]DirectoryStudent, error) {
	students, err := d.Directory.ListParentPortalStudents(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]DirectoryStudent, 0, len(students))
	for _, student := range students {
		result = append(result, DirectoryStudent{ID: student.ID, TenantID: student.TenantID, PersonID: student.PersonID, SchoolClass: student.SchoolClass, Status: student.Status, EnrolledFrom: student.EnrolledFrom.String(), EnrolledUntil: student.EnrolledUntil.String()})
	}
	return result, nil
}

type ParentGuardianDirectory struct {
	Directory careplan.ParentPortalDirectory
}

func (d ParentGuardianDirectory) ListGuardianLinksByAccount(ctx context.Context, accountID int64) ([]DirectoryGuardianLink, error) {
	links, err := d.Directory.ListParentPortalGuardianLinks(ctx, accountID)
	if err != nil {
		return nil, err
	}
	result := make([]DirectoryGuardianLink, 0, len(links))
	for _, link := range links {
		result = append(result, DirectoryGuardianLink(link))
	}
	return result, nil
}
