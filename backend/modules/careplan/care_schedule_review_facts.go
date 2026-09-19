package careplan

import (
	"encoding/json"
	"fmt"
	"strings"
)

func CareReviewUrgentOn(item CareScheduleReviewItem, today Date) bool {
	if item.Request.RequestKind == "pickup_change" {
		return careReviewPickupDate(item) == today.String()
	}
	weekday := int(today.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	for _, diff := range item.Diff {
		if diff.Weekday == weekday {
			return true
		}
	}
	return false
}

func CareReviewPastOn(item CareScheduleReviewItem, today Date) bool {
	if item.Request.RequestKind != "pickup_change" {
		return false
	}
	date, err := ParseDate(careReviewPickupDate(item))
	return err == nil && date.Before(today)
}

func CareReviewConflictKeys(item CareScheduleReviewItem) []string {
	if item.Request.RequestKind == "pickup_change" {
		if date := careReviewPickupDate(item); date != "" {
			return []string{"pickup:" + date}
		}
		return nil
	}
	keys := make([]string, 0, len(item.Diff))
	for _, diff := range item.Diff {
		if diff.Weekday < 1 || diff.Weekday > 7 {
			continue
		}
		kind := strings.TrimSpace(diff.CareKind)
		if kind == "" {
			kind = "plan"
		}
		keys = append(keys, fmt.Sprintf("care:%d:%s", diff.Weekday, kind))
	}
	return keys
}

func careReviewPickupDate(item CareScheduleReviewItem) string {
	var payload struct {
		Date string `json:"date"`
	}
	_ = json.Unmarshal(item.Request.Payload, &payload)
	return payload.Date
}
