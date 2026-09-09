package facilities

import "time"

// Room is the ORM compatibility row retained for legacy relations and test
// fixtures. Runtime room reads and writes go through modules/facilities.
type Room struct {
	//nolint:unused // BUN consumes this table metadata through reflection.
	tableName struct{}  `bun:"table:facilities.rooms,alias:room"`
	ID        int64     `bun:"id,pk,autoincrement" json:"id"`
	TenantID  int64     `bun:"tenant_id,notnull" json:"tenant_id"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
	Name      string    `bun:"name,notnull" json:"name"`
	Building  string    `bun:"building" json:"building,omitempty"`
	Floor     *int      `bun:"floor" json:"floor,omitempty"`
	Capacity  *int      `bun:"capacity" json:"capacity,omitempty"`
	Category  *string   `bun:"category" json:"category,omitempty"`
	Color     *string   `bun:"color" json:"color,omitempty"`
	IsSystem  bool      `bun:"is_system,notnull,default:false" json:"is_system"`
	// IsOpenRoom reports whether the OGS administration has permanently
	// released this room for use ("offener Raum", #3062). Facilities owns the
	// value; this row only carries it for the legacy relations and fixtures
	// that have not moved onto the module's public capability yet.
	IsOpenRoom bool `bun:"is_open_room,notnull,default:false" json:"is_open_room"`
}

func (r *Room) SetTenantID(id int64) { r.TenantID = id }
func (r *Room) GetTenantID() int64   { return r.TenantID }
