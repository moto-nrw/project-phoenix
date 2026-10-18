package presenceprojectionhttp

import "github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"

func sortExportResponses(students []StudentResponse, mode string) {
	testResource(ResourceConfig{}).sortExportResponses(students, mode)
}
func groupExportResponsesByClass(students []StudentResponse) {
	testResource(ResourceConfig{}).groupExportResponsesByClass(students)
}
func buildExportRowSources(sources []exportRowSource, grouped bool) []lists.Row {
	return testResource(ResourceConfig{}).buildExportRowSources(sources, grouped)
}
