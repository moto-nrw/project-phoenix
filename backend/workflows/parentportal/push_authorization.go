package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

type DocumentPush = care.DocumentPush
type DocumentPushStudent = care.DocumentPushStudent

type DocumentPushAccount = care.DocumentPushAccount
type DocumentPushRelationship = care.DocumentPushRelationship
type DocumentPushAccounts = care.DocumentPushAccounts
type DocumentPushRelationships = care.DocumentPushRelationships
type DocumentPushStudents = care.DocumentPushStudents
type DocumentPushCheck = care.DocumentPushCheck
type DocumentPushObjects = care.DocumentPushObjects

type DocumentPushAuthorization interface {
	AuthorizeDocumentPushes(context.Context, []DocumentPush) (map[int64]bool, error)
}

type DocumentPushAuthorizationDependencies = care.DocumentPushAuthorizationDependencies
