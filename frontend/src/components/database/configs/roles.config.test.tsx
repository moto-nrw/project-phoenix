/**
 * Tests for Roles Configuration
 * Tests role config structure and response mapping
 */
import { describe, it, expect, vi } from "vitest";
import { roleEditSections, rolesConfig } from "./roles.config";
import type { Role } from "@/lib/auth-helpers";

// Mock auth-helpers
vi.mock("@/lib/auth-helpers", () => ({
  mapRoleResponse: vi.fn((data: unknown) => data),
  getRoleDisplayName: vi.fn((name: string) => name),
  getRoleDisplayDescription: vi.fn((_name: string, desc: string) => desc),
  getBaseRoleLabel: vi.fn((role?: string) => role ?? "Keine Zuordnung"),
  BASE_ROLE_LABELS: {
    admin: "Verwaltung",
    user: "Betreuung",
    guardian: "Eltern",
  },
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

  it("creates a role from a template and edits its tier (#4574)", () => {
    const createFields = rolesConfig.form.sections[0]?.fields ?? [];
    expect(createFields.map((f) => f.name)).toEqual([
      "name",
      "description",
      "template",
    ]);
    expect(createFields.find((f) => f.name === "template")).toMatchObject({
      label: "Vorlage",
      required: true,
      options: [
        { value: "user", label: "Wie Betreuung" },
        { value: "admin", label: "Wie Verwaltung" },
        { value: "lehrkraft", label: "Wie Lehrkraft" },
      ],
    });

    const editFields = roleEditSections[0]?.fields ?? [];
    expect(editFields.map((f) => f.name)).toEqual([
      "name",
      "description",
      "baseRole",
    ]);
    expect(editFields.find((f) => f.name === "baseRole")?.label).toBe(
      "Gilt als",
    );
  });

  it("does not offer Eltern under „Gilt als“ (#4574)", () => {
    const options = roleEditSections[0]?.fields.find(
      (f) => f.name === "baseRole",
    )?.options;

    expect(options).toEqual([
      { value: "admin", label: "Verwaltung" },
      { value: "user", label: "Betreuung" },
    ]);
  });

  it("sends the template instead of base_role when creating", () => {
    const mapRequest = rolesConfig.service?.mapRequest;
    expect(
      mapRequest?.({
        name: "Springkraft",
        description: "",
        template: "user",
      } as Partial<Role>),
    ).toEqual({ name: "Springkraft", description: "", template: "user" });
    expect(
      mapRequest?.({ name: "Springkraft", description: "", baseRole: "admin" }),
    ).toEqual({ name: "Springkraft", description: "", base_role: "admin" });
  });

  it("has detail header configuration", () => {
    const mockRole: Role = {
      id: "1",
      name: "admin",
      description: "Administrator role",
      isSystem: false,
      permissions: [],
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };

    expect(rolesConfig.detail.header?.title(mockRole)).toBe("admin");
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
      isSystem: false,
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
