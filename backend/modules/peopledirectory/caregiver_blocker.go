package peopledirectory

// BlockerActivity represents an activity supervision assignment that blocks disable.
type BlockerActivity struct {
	ID           int64  `json:"id"`
	ActivityID   int64  `json:"activity_id"`
	ActivityName string `json:"activity_name"`
	IsPrimary    bool   `json:"is_primary"`
}
