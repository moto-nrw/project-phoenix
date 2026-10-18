package careplanhttp

import (
	careplan "github.com/moto-nrw/project-phoenix/modules/careplan"
	departure "github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
)

// toCompanionLinks converts the wire entries into the service's link shape.
func toCompanionLinks(entries []CompanionEntry) []departure.CompanionLink {
	links := make([]departure.CompanionLink, 0, len(entries))
	for _, entry := range entries {
		links = append(links, departure.CompanionLink{
			CompanionStudentID: entry.CompanionStudentID,
			Weekdays:           entry.Weekdays,
		})
	}
	return links
}

func toCompanionResponses(links []departure.CompanionLink) []CompanionResponse {
	out := make([]CompanionResponse, 0, len(links))
	for _, link := range links {
		weekdays := link.Weekdays
		if weekdays == nil {
			weekdays = []string{}
		}
		out = append(out, CompanionResponse{
			CompanionStudentID: link.CompanionStudentID,
			FirstName:          link.FirstName,
			LastName:           link.LastName,
			Weekdays:           weekdays,
		})
	}
	return out
}

// carePlanCompanionLinks and peopleCompanionLinks translate the "läuft mit"
// links at the seam between Care Plan, which owns them, and the People
// Directory row whose departure plan they answer for. A nil list stays nil.
func carePlanCompanionLinks(links []departure.CompanionLink) []careplan.CompanionLink {
	if links == nil {
		return nil
	}
	result := make([]careplan.CompanionLink, 0, len(links))
	for _, link := range links {
		result = append(result, careplan.CompanionLink(link))
	}
	return result
}

func peopleCompanionLinks(links []careplan.CompanionLink) []departure.CompanionLink {
	if links == nil {
		return nil
	}
	result := make([]departure.CompanionLink, 0, len(links))
	for _, link := range links {
		result = append(result, departure.CompanionLink(link))
	}
	return result
}
