package api

import owners "github.com/moto-nrw/project-phoenix/roots/owners"

// newDeclarationReports binds the report documents of the Erklärung proofs
// (#3430), Care Plan's proofs and the weekly protocol (#4769) to Document
// Rendering's record renderer; the Worker root uses the same binding.
func newDeclarationReports() owners.RecordReports { return owners.NewRecordReports() }

func reportText(text string) string { return owners.ReportText(text) }
