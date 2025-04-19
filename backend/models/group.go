// Package models contains application specific entities.
package models

import (
	"time"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/uptrace/bun"
)

// Group represents a group of students
type Group struct {
	ID               int64                    `json:"id" bun:"id,pk,autoincrement"`
	Name             string                   `json:"name" bun:"name,notnull,unique"`
	RoomID           *int64                   `json:"room_id,omitempty" bun:"room_id"`
	Room             *Room                    `json:"room,omitempty" bun:"rel:belongs-to,join:room_id=id"`
	RepresentativeID *int64                   `json:"representative_id,omitempty" bun:"representative_id"`
	Representative   *PedagogicalSpecialist   `json:"representative,omitempty" bun:"rel:belongs-to,join:representative_id=id"`
	Students         []Student                `json:"students,omitempty" bun:"rel:has-many,join:id=group_id"`
	Supervisors      []*PedagogicalSpecialist `json:"supervisors,omitempty" bun:"m2m:group_supervisors,join:Group=Specialist"`
	CreatedAt        time.Time                `json:"created_at" bun:"created_at,notnull"`
	ModifiedAt       time.Time                `json:"updated_at" bun:"modified_at,notnull"`
}

// BeforeInsert hook executed before database insert operation.
func (g *Group) BeforeInsert(db *bun.DB) error {
	now := time.Now()
	g.CreatedAt = now
	g.ModifiedAt = now
	return g.Validate()
}

// BeforeUpdate hook executed before database update operation.
func (g *Group) BeforeUpdate(db *bun.DB) error {
	g.ModifiedAt = time.Now()
	return g.Validate()
}

// Validate validates Group struct and returns validation errors.
func (g *Group) Validate() error {
	return validation.ValidateStruct(g,
		validation.Field(&g.Name, validation.Required),
	)
}

// GroupSupervisor is the junction table for many-to-many relationship between Group and PedagogicalSpecialist
type GroupSupervisor struct {
	ID           int64                  `bun:"id,pk,autoincrement"`
	GroupID      int64                  `bun:"group_id,notnull"`
	Group        *Group                 `bun:"rel:belongs-to,join:group_id=id"`
	SpecialistID int64                  `bun:"specialist_id,notnull"`
	Specialist   *PedagogicalSpecialist `bun:"rel:belongs-to,join:specialist_id=id"`
	CreatedAt    time.Time              `bun:"created_at,notnull"`

	bun.BaseModel `bun:"table:group_supervisors"`
}

// BeforeInsert hook executed before database insert operation.
func (gs *GroupSupervisor) BeforeInsert(db *bun.DB) error {
	gs.CreatedAt = time.Now()
	return nil
}

// CombinedGroup represents a temporary combination of multiple groups
type CombinedGroup struct {
	ID                int64                    `json:"id" bun:"id,pk,autoincrement"`
	Name              string                   `json:"name" bun:"name,notnull,unique"`
	IsActive          bool                     `json:"is_active" bun:"is_active,notnull,default:true"`
	CreatedAt         time.Time                `json:"created_at" bun:"created_at,notnull"`
	ValidUntil        *time.Time               `json:"valid_until,omitempty" bun:"valid_until"`
	AccessPolicy      string                   `json:"access_policy" bun:"access_policy,notnull"`
	SpecificGroupID   *int64                   `json:"specific_group_id,omitempty" bun:"specific_group_id"`
	SpecificGroup     *Group                   `json:"specific_group,omitempty" bun:"rel:belongs-to,join:specific_group_id=id"`
	Groups            []*Group                 `json:"groups,omitempty" bun:"m2m:combined_group_groups,join:CombinedGroup=Group"`
	AccessSpecialists []*PedagogicalSpecialist `json:"access_specialists,omitempty" bun:"m2m:combined_group_specialists,join:CombinedGroup=Specialist"`
}

// BeforeInsert hook executed before database insert operation.
func (cg *CombinedGroup) BeforeInsert(db *bun.DB) error {
	cg.CreatedAt = time.Now()
	return cg.Validate()
}

// Validate validates CombinedGroup struct and returns validation errors.
func (cg *CombinedGroup) Validate() error {
	return validation.ValidateStruct(cg,
		validation.Field(&cg.Name, validation.Required),
		validation.Field(&cg.AccessPolicy, validation.Required, validation.In("all", "first", "specific", "manual")),
	)
}

// CombinedGroupGroup is the junction table for many-to-many relationship between CombinedGroup and Group
type CombinedGroupGroup struct {
	ID              int64          `bun:"id,pk,autoincrement"`
	CombinedGroupID int64          `bun:"combinedgroup_id,notnull"`
	CombinedGroup   *CombinedGroup `bun:"rel:belongs-to,join:combinedgroup_id=id"`
	GroupID         int64          `bun:"group_id,notnull"`
	Group           *Group         `bun:"rel:belongs-to,join:group_id=id"`
	CreatedAt       time.Time      `bun:"created_at,notnull"`

	bun.BaseModel `bun:"table:combined_group_groups"`
}

// BeforeInsert hook executed before database insert operation.
func (cgg *CombinedGroupGroup) BeforeInsert(db *bun.DB) error {
	cgg.CreatedAt = time.Now()
	return nil
}

// CombinedGroupSpecialist is the junction table for many-to-many relationship between CombinedGroup and PedagogicalSpecialist
type CombinedGroupSpecialist struct {
	ID              int64                  `bun:"id,pk,autoincrement"`
	CombinedGroupID int64                  `bun:"combinedgroup_id,notnull"`
	CombinedGroup   *CombinedGroup         `bun:"rel:belongs-to,join:combinedgroup_id=id"`
	SpecialistID    int64                  `bun:"specialist_id,notnull"`
	Specialist      *PedagogicalSpecialist `bun:"rel:belongs-to,join:specialist_id=id"`
	CreatedAt       time.Time              `bun:"created_at,notnull"`

	bun.BaseModel `bun:"table:combined_group_specialists"`
}

// BeforeInsert hook executed before database insert operation.
func (cgs *CombinedGroupSpecialist) BeforeInsert(db *bun.DB) error {
	cgs.CreatedAt = time.Now()
	return nil
}
