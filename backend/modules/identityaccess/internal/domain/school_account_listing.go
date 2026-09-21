package domain

// SchoolAccountListing is an account mapping or a pending invitation at one school.
type SchoolAccountListing struct {
	SchoolID     int64
	AccountID    int64
	Email        string
	Active       bool
	FirstName    string
	LastName     string
	RoleName     string
	Status       string
	HasAdminRole bool
	HasUserRole  bool
}
