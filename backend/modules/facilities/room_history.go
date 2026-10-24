package facilities

import "time"

type RoomSessionEntry struct {
	SessionID       int64      `json:"session_id"`
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	DurationMinutes *int       `json:"duration_minutes,omitempty"`
	ActivityName    string     `json:"activity_name"`
	SupervisorName  string     `json:"supervisor_name"`
	StudentCount    int        `json:"student_count"`
}
