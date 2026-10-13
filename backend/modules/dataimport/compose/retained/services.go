// Package retained names the retained Data Import application services for
// the legacy composition (#2747). The application package imports the Data
// Import composition itself, so the names live one level down instead of in
// it.
package retained

import importService "github.com/moto-nrw/project-phoenix/services/import"

type (
	ImportService[T any]        = importService.ImportService[T]
	ImportObservation           = importService.ImportObservation
	ImportRuntime               = importService.ImportRuntime
	StudentImportDeps           = importService.StudentImportDeps
	StaffImportDeps             = importService.StaffImportDeps
	ClassListImportDeps         = importService.ClassListImportDeps
	OpeningBalanceImportDeps    = importService.OpeningBalanceImportDeps
	OpeningBalanceImportFactory = importService.OpeningBalanceImportFactory
	GuardianPort                = importService.GuardianPort
	GuardianPortDecorator       = importService.GuardianPortDecorator
)

var (
	NewStudentImportConfig        = importService.NewStudentImportConfig
	NewStaffImportConfig          = importService.NewStaffImportConfig
	NewClassListImportConfig      = importService.NewClassListImportConfig
	NewOpeningBalanceImportConfig = importService.NewOpeningBalanceImportConfig
	NewRelationshipResolver       = importService.NewRelationshipResolver
	MapRelationshipType           = importService.MapRelationshipType
)

// NewImportService builds the batch import service for one row type.
func NewImportService[T any](config importService.ImportConfig[T], runtime ImportRuntime) *ImportService[T] {
	return importService.NewImportService(config, runtime)
}
