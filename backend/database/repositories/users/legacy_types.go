package users

import userModels "github.com/moto-nrw/project-phoenix/models/users"

// These aliases let compatibility adapters depend on the People Directory
// repository boundary instead of its internal model package.
type StudentCompanion = userModels.StudentCompanion
type CompanionLink = userModels.CompanionLink
type StudentDocument = userModels.StudentDocument
type StudentDocumentFileCleanup = userModels.StudentDocumentFileCleanup
type StudentCompanionRepository = userModels.StudentCompanionRepository
type StudentDocumentRepository = userModels.StudentDocumentRepository
type CareExit = userModels.CareExit

var CompanionWeekdayKeys = userModels.CompanionWeekdayKeys
var ErrCompanionInvalidWeekday = userModels.ErrCompanionInvalidWeekday
