package test

import (
	"time"

	"github.com/uptrace/bun"
)

// Room is one facilities.rooms row the fixtures write. The Facilities owner
// keeps its row mapping in models/facilities, which the shared fixtures may
// not import (#2748), so test support maps the table itself.
type Room struct {
	bun.BaseModel `bun:"table:facilities.rooms,alias:room"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	Name          string    `bun:"name,notnull"`
	Building      string    `bun:"building"`
	Floor         *int      `bun:"floor"`
	Capacity      *int      `bun:"capacity"`
	Category      *string   `bun:"category"`
	Color         *string   `bun:"color"`
	IsSystem      bool      `bun:"is_system,notnull,default:false"`
	IsOpenRoom    bool      `bun:"is_open_room,notnull,default:false"`
}
