/**
 * Tests for Roles Configuration
 * Tests role config structure and response mapping
 */
import { describe, it, expect, vi } from "vitest";
import { rolesConfig } from "./roles.config";
import type { Role } from "@/lib/auth-helpers";

// Mock auth-helpers
vi.mock("@/lib/auth-helpers", () => ({
  mapRoleResponse: vi.fn((data: unknown) => data),
  getRoleDisplayName: vi.fn((name: string) => name),
  getRoleDisplayDescription: vi.fn((_name: string, desc: string) => desc),
}));

describe("rolesConfig", () => {
  it("exports a valid entity config", () => {
    expect(rolesConfig).toBeDefined();
    expect(rolesConfig.name).toEqual({
      singular: "Rolle",
      plural: "Rollen",
    });
  });

  it("has correct API configuration", () => {
    expect(rolesConfig.api.basePath).toBe("/api/auth/roles");
  });

  it("has form sections configured", () => {
    expect(rolesConfig.form.sections).toHaveLength(1);
    expect(rolesConfig.form.sections[0]?.title).toBe("Rolleninformationen");
  });

  it("has required form fields", () => {
    const fields = rolesConfig.form.sections[0]?.fields ?? [];
    const fieldNames = fields.map((f) => f.name);

    expect(fieldNames).toContain("name");
    expect(fieldNames).toContain("description");
  });

  it("has detail header configuration", () => {
    const mockRole: Role = {
      id: "1",
      name: "admin",
      description: "Administrator role",
      permissions: [],
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };

    expect(rolesConfig.detail.header?.title(mockRole)).toBe("admin");
  });

  it("shows permission count badge", () => {
    const mockRole: Role = {
      id: "1",
      name: "admin",
      description: "Administrator role",
      permissions: [
        {
          id: "1",
          name: "perm1",
          resource: "students",
          action: "read",
          description: "",
          createdAt: "",
          updatedAt: "",
        },
        {
          id: "2",
          name: "perm2",
          resource: "rooms",
          action: "write",
          description: "",
          createdAt: "",
          updatedAt: "",
        },
      ],
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };

    const badges = rolesConfig.detail.header?.badges ?? [];
    const permBadge = badges[0];
    expect((permBadge?.label as (entity: Role) => string)(mockRole)).toBe(
      "2 Berechtigungen",
    );
  });

  it("has list configuration", () => {
    expect(rolesConfig.list.title).toBe("Rollen verwalten");
    expect(rolesConfig.list.searchStrategy).toBe("frontend");
  });

  it("displays role in list", () => {
    const mockRole: Role = {
      id: "1",
      name: "admin",
      description: "Administrator role",
      permissions: [],
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };

    const title = rolesConfig.list.item.title(mockRole);
    expect(title).toBe("admin");
  });

  it("has custom labels", () => {
    expect(rolesConfig.labels?.createButton).toBe("Neue Rolle erstellen");
    expect(rolesConfig.labels?.deleteConfirmation).toContain("löschen");
  });
});
