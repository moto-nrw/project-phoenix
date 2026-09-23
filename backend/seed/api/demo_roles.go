package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

// The reduced roles of the demo school profile (#3469). A visitor of the
// public demo sees the school as one of them; "Alle Funktionen" is the
// administrator. The names are the contract with the demo role switch
// (modules/identityaccess/internal/domain/demo_access.go: DemoSchoolRole*).
// Both roles are part of the profile, so the local seed has them as well;
// its staff accounts keep the system roles as before.
const (
	demoRoleCaregiverName = "betreuungskraft"
	demoRoleLeadName      = "ogs-leitung"
)

type demoRoleDefinition struct {
	Name        string
	Description string
	BaseRole    string
	Permissions []string
}

// demoRoleDefinitions lists the permission sets. Every name must exist in
// the permission catalog; the seeder refuses an unknown one, so the sets
// cannot silently shrink when a permission is renamed.
//
// Betreuungskraft is the day in the group: children, rooms, activities, the
// own supervision, calendar and working time, and the requests of parents.
// It is the standard staff role without creating children.
//
// OGS-Leitung adds planning and administration: the care plan, shifts and
// substitutions, master data of children, staff, rooms, groups and
// activities, enrollment and settings, files and documents. It has no
// system administration: no roles, permissions, devices, info displays or
// grade transitions, and no admin wildcard. It also lacks users:manage,
// which would let it hand out any role including the administrator
// (authorize.CanGrantRole); staff invitations and parent-portal approvals
// stay with "Alle Funktionen".
func demoRoleDefinitions() []demoRoleDefinition {
	return []demoRoleDefinition{
		{
			Name:        demoRoleCaregiverName,
			Description: "Meldet Kinder an und ab, betreut die eigene Gruppe und beantwortet Anfragen der Eltern.",
			BaseRole:    "user",
			Permissions: []string{
				"activities:assign", "activities:create", "activities:delete", "activities:enroll",
				"activities:list", "activities:manage", "activities:read", "activities:update",
				"calendar:own",
				"feedback:list", "feedback:read",
				"groups:list", "groups:read", "groups:update",
				"rooms:list", "rooms:read",
				"schedules:list", "schedules:read",
				"staff_notices:read",
				"substitutions:read",
				"supervision:own",
				"time_tracking:own",
				"users:absence", "users:checkin", "users:list", "users:read", "users:update",
				"visits:create", "visits:delete", "visits:list", "visits:read", "visits:update",
			},
		},
		{
			Name:        demoRoleLeadName,
			Description: "Plant Betreuung und Dienste, pflegt Kinder, Personal und Räume, verwaltet Anmeldungen und Einstellungen.",
			BaseRole:    "admin",
			Permissions: []string{
				"activities:assign", "activities:create", "activities:delete", "activities:enroll",
				"activities:list", "activities:manage", "activities:manage_categories", "activities:read", "activities:update",
				"calendar:manage", "calendar:own",
				"config:manage", "config:read", "config:update",
				"feedback:create", "feedback:delete", "feedback:list", "feedback:manage", "feedback:read",
				"files:manage",
				"groups:assign", "groups:create", "groups:delete", "groups:list", "groups:manage", "groups:read", "groups:update",
				"guardians:financial",
				"rooms:create", "rooms:delete", "rooms:list", "rooms:manage", "rooms:read", "rooms:update",
				"schedules:create", "schedules:delete", "schedules:list", "schedules:manage", "schedules:read", "schedules:update",
				"staff:documents", "staff:financial", "staff:manage", "staff:stammdaten",
				"staff_documents:health",
				"staff_notices:read",
				"student_documents:health", "student_documents:legal",
				"substitutions:create", "substitutions:delete", "substitutions:list", "substitutions:manage", "substitutions:read", "substitutions:update",
				"supervision:own",
				"time_tracking:manage", "time_tracking:own",
				"users:absence", "users:checkin", "users:create", "users:delete", "users:list", "users:read", "users:update",
				"vacation:approve",
				"visits:create", "visits:delete", "visits:list", "visits:manage", "visits:read", "visits:update",
			},
		},
	}
}

// seedDemoRoles creates the reduced roles of the school with their
// permission sets and remembers their ids next to the system roles.
func (s *FixedSeeder) seedDemoRoles(_ context.Context) error {
	permissionIDs, err := s.fetchPermissionIDs()
	if err != nil {
		return err
	}
	for _, definition := range demoRoleDefinitions() {
		ids := make([]string, 0, len(definition.Permissions))
		for _, name := range definition.Permissions {
			id, ok := permissionIDs[name]
			if !ok {
				return fmt.Errorf("demo role %s: permission %q is not in the catalog", definition.Name, name)
			}
			ids = append(ids, strconv.FormatInt(id, 10))
		}
		created, err := s.client.Post("/auth/roles", map[string]any{
			"name": definition.Name, "description": definition.Description, "base_role": definition.BaseRole,
		})
		if err != nil {
			return fmt.Errorf("create demo role %s: %w", definition.Name, err)
		}
		var role struct {
			Data struct {
				ID int64 `json:"id,string"`
			} `json:"data"`
		}
		if err := json.Unmarshal(created, &role); err != nil {
			return fmt.Errorf("parse demo role %s: %w", definition.Name, err)
		}
		if role.Data.ID <= 0 {
			return fmt.Errorf("demo role %s was created without an id", definition.Name)
		}
		if _, err := s.client.Put(fmt.Sprintf("/auth/roles/%d/permissions", role.Data.ID), map[string]any{"permission_ids": ids}); err != nil {
			return fmt.Errorf("grant permissions of demo role %s: %w", definition.Name, err)
		}
		s.roleIDs[definition.Name] = role.Data.ID
	}
	if s.verbose {
		fmt.Printf("  ✓ %d demo roles created\n", len(demoRoleDefinitions()))
	}
	return nil
}

// fetchPermissionIDs reads the permission catalog as "resource:action" -> id.
func (s *FixedSeeder) fetchPermissionIDs() (map[string]int64, error) {
	respBody, err := s.client.Get("/auth/permissions")
	if err != nil {
		return nil, fmt.Errorf("fetch permissions: %w", err)
	}
	var resp struct {
		Data []struct {
			ID       int64  `json:"id,string"`
			Resource string `json:"resource"`
			Action   string `json:"action"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parse permissions response: %w", err)
	}
	ids := make(map[string]int64, len(resp.Data))
	for _, permission := range resp.Data {
		ids[permission.Resource+":"+permission.Action] = permission.ID
	}
	return ids, nil
}
