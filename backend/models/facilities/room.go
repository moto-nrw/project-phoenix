package facilities

import (
	"time"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
)

// Room is the ORM compatibility row retained for legacy relations and test
// fixtures. Runtime room reads and writes go through modules/facilities.
type Room struct {
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
}

var (
	ErrNameRequired        = facilitiesModule.ErrNameRequired
	ErrCapacityNotPositive = facilitiesModule.ErrCapacityNotPositive
	ErrInvalidColorFormat  = facilitiesModule.ErrInvalidColorFormat
	ErrReservedColor       = facilitiesModule.ErrReservedColor
)

func (r *Room) GetID() interface{}      { return r.ID }
func (r *Room) GetCreatedAt() time.Time { return r.CreatedAt }
func (r *Room) GetUpdatedAt() time.Time { return r.UpdatedAt }
func (r *Room) SetTenantID(id int64)    { r.TenantID = id }
func (r *Room) GetTenantID() int64      { return r.TenantID }

func (r *Room) IsAvailable(required int) bool {
	return r.Capacity == nil || *r.Capacity <= 0 || *r.Capacity >= required
}

func (r *Room) GetFullName() string {
	if r.Building != "" {
		return r.Building + " - " + r.Name
	}
	return r.Name
}

func (r *Room) Validate() error {
	public := r.public()
	if err := public.Validate(); err != nil {
		return err
	}
	r.Name, r.Color = public.Name, public.Color
	return nil
}

func (r *Room) public() facilitiesModule.Room {
	return facilitiesModule.Room{
		ID: r.ID, TenantID: r.TenantID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		Name: r.Name, Building: r.Building, Floor: r.Floor, Capacity: r.Capacity,
		Category: r.Category, Color: r.Color, IsSystem: r.IsSystem,
	}
}

func IsValidationError(err error) bool { return facilitiesModule.IsValidationError(err) }
