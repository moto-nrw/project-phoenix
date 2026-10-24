package requestreview

import "strings"

func GermanOfferingDiffLabel(state string, days []string) string {
	switch state {
	case "not_booked":
		return "nicht gebucht"
	case "removed":
		return "abgemeldet"
	}
	if len(days) == 0 {
		return "alle Betreuungstage"
	}
	labels := map[string]string{"mon": "Mo", "tue": "Di", "wed": "Mi", "thu": "Do", "fri": "Fr", "sat": "Sa", "sun": "So"}
	parts := make([]string, 0, len(days))
	for _, day := range days {
		if label, found := labels[day]; found {
			parts = append(parts, label)
		}
	}
	return strings.Join(parts, ", ")
}
