package domain

// TraegerAccount is an account at a Träger with its Träger roles (ADR 0084).
type TraegerAccount struct {
	AccountID int64
	Email     string
	Roles     []string
}
