package persistence

import "github.com/uptrace/bun"

type Record struct {
	bun.BaseModel `bun:"table:beta.hidden_records"`
}
