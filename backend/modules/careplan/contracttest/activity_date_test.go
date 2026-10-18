package contracttest_test

import "time"

func activityDatePtr[D ~string](date *D) *string {
	if date == nil {
		return nil
	}
	value := string(*date)
	return &value
}

func activityClockPtr(value *time.Time) *string {
	if value == nil {
		return nil
	}
	text := value.Format("15:04:05")
	return &text
}
