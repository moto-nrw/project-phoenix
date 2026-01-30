/**
 * Tests for Permissions Configuration
 * Tests permission config structure and transform functions
 */
import { describe, it, expect, vi } from "vitest";
import { permissionsConfig } from "./permissions.config";
import type { Permission } from "@/lib/auth-helpers";

// Mock modules
vi.mock("@/lib/auth-helpers", () => ({
  mapPermissionResponse: vi.fn((data: unknown) => data),
}));

vi.mock("@/lib/permission-labels", () => ({
  formatPermissionDisplay: vi.fn((resource, action) => `${resource}:${action}`),
}));

vi.mock("@/components/permissions/permission-selector", () => ({
  PermissionSelector: vi.fn(() => null),
  RESOURCES: [
    { value: "students", label: "Schüler" },
    { value: "rooms", label: "Räume" },
  ],
  ACTION_LABELS: {
    read: "Lesen",
    write: "Schreiben",
    delete: "Löschen",
  },
}));

describe("permissionsConfig", () => {
  it("exports a valid entity config", () => {
    expect(permissionsConfig).toBeDefined();
    expect(permissionsConfig.name).toEqual({
      singular: "Berechtigung",
      plural: "Berechtigungen",
    });
  });

  it("has correct API configuration", () => {
    expect(permissionsConfig.api.basePath).toBe("/api/auth/permissions");
  });

  it("has form sections configured", () => {
    expect(permissionsConfig.form.sections).toHaveLength(1);
    expect(permissionsConfig.form.sections[0]?.title).toBe(
      "Berechtigungsdetails",
    );
  });

  it("has required form fields", () => {
    const fields = permissionsConfig.form.sections[0]?.fields ?? [];
    const fieldNames = fields.map((f) => f.name);

    expect(fieldNames).toContain("permissionSelector");
    expect(fieldNames).toContain("name");
    expect(fieldNames).toContain("description");
  });

  it("transforms data before submit with auto-generated name", () => {
    const data = {
      permissionSelector: {
        resource: "students",
        action: "read",
      },
      description: "Test permission",
    };

    const transformed = permissionsConfig.form.transformBeforeSubmit?.(data);
    expect(transformed?.name).toBe("schüler:lesen");
    expect(transformed?.resource).toBe("students");
    expect(transformed?.action).toBe("read");
  });

  it("preserves provided name in transform", () => {
    const data = {
      permissionSelector: {
        resource: "students",
        action: "read",
      },
      name: "Custom Name",
      description: "Test permission",
    };

    const transformed = permissionsConfig.form.transformBeforeSubmit?.(data);
    expect(transformed?.name).toBe("Custom Name");
  });

  it("has detail header configuration", () => {
    const mockPermission: Permission = {
      id: "1",
      name: "students:read",
      description: "Read students",
      resource: "students",
      action: "read",
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };

    expect(permissionsConfig.detail.header?.title(mockPermission)).toBe(
      "students:read",
    );
  });

  it("shows no description message when not provided", () => {
    const mockPermission = {
      id: "1",
      name: "students:read",
      resource: "students",
      action: "read",
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    } as unknown as Permission;

    expect(permissionsConfig.detail.header?.subtitle?.(mockPermission)).toBe(
      "Keine Beschreibung",
    );
  });

  it("has list configuration", () => {
    expect(permissionsConfig.list.title).toBe("Berechtigungen verwalten");
    expect(permissionsConfig.list.searchStrategy).toBe("frontend");
  });

  it("displays permission in list", () => {
    const mockPermission: Permission = {
      id: "1",
      name: "students:read",
      description: "Read students",
      resource: "students",
      action: "read",
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };

    const title = permissionsConfig.list.item.title(mockPermission);
    expect(title).toBe("students:read");
  });

  it("has custom labels", () => {
    expect(permissionsConfig.labels?.createButton).toBe(
      "Neue Berechtigung erstellen",
    );
    expect(permissionsConfig.labels?.deleteConfirmation).toContain("löschen");
  });
});
