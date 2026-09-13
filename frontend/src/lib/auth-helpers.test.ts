import { describe, it, expect } from "vitest";
import {
  getRoleDisplayName,
  getRoleDisplayDescription,
  toAssignableRoleOptions,
  type Role,
} from "./auth-helpers";

describe("getRoleDisplayName", () => {
  it("translates admin to Administrator", () => {
    expect(getRoleDisplayName("admin")).toBe("Administrator");
  });

  it("translates Admin (case-insensitive) to Administrator", () => {
    expect(getRoleDisplayName("Admin")).toBe("Administrator");
  });

  it("translates ADMIN (uppercase) to Administrator", () => {
    expect(getRoleDisplayName("ADMIN")).toBe("Administrator");
  });

  it("translates user to Betreuer", () => {
    expect(getRoleDisplayName("user")).toBe("Betreuer");
  });

  it("translates guest to Gast", () => {
    expect(getRoleDisplayName("guest")).toBe("Gast");
  });

  it("returns original name for removed roles like teacher", () => {
    expect(getRoleDisplayName("teacher")).toBe("teacher");
  });

  it("returns original name for removed roles like staff", () => {
    expect(getRoleDisplayName("staff")).toBe("staff");
  });

  it("translates guardian to Erziehungsberechtigter", () => {
    expect(getRoleDisplayName("guardian")).toBe("Erziehungsberechtigter");
  });

  it("returns original name for non-system roles", () => {
    expect(getRoleDisplayName("custom_role")).toBe("custom_role");
  });
});

describe("getRoleDisplayDescription", () => {
  it("translates admin description to German", () => {
    expect(getRoleDisplayDescription("admin", "System administrator")).toBe(
      "Systemadministrator mit vollem Zugriff",
    );
  });

  it("translates user description to Betreuer description", () => {
    expect(getRoleDisplayDescription("user", "Standard user")).toBe(
      "Pädagogische Fachkraft mit Betreuungsrechten",
    );
  });

  it("translates guest description to German", () => {
    expect(getRoleDisplayDescription("guest", "Limited access")).toBe(
      "Eingeschränkter Zugriff für nicht authentifizierte Benutzer",
    );
  });

  it("returns original description for removed roles like teacher", () => {
    expect(getRoleDisplayDescription("teacher", "Teaching staff")).toBe(
      "Teaching staff",
    );
  });

  it("returns original description for removed roles like staff", () => {
    expect(getRoleDisplayDescription("staff", "General staff")).toBe(
      "General staff",
    );
  });

  it("translates guardian description to German", () => {
    expect(getRoleDisplayDescription("guardian", "Parent access")).toBe(
      "Eingeschränkter Zugriff auf Daten der eigenen Kinder",
    );
  });

  it("returns original description for non-system roles", () => {
    expect(getRoleDisplayDescription("custom", "Custom description")).toBe(
      "Custom description",
    );
  });

  it("handles case-insensitive role names", () => {
    expect(getRoleDisplayDescription("ADMIN", "Admin")).toBe(
      "Systemadministrator mit vollem Zugriff",
    );
  });
});

function role(id: string, name: string, isSystem: boolean): Role {
  return {
    id,
    name,
    description: "",
    isSystem,
    createdAt: "",
    updatedAt: "",
  };
}

describe("toAssignableRoleOptions", () => {
  it("keeps tenant roles whose names match excluded system roles", () => {
    const options = toAssignableRoleOptions([
      role("1", "teacher", true),
      role("2", "guardian", true),
      role("3", "teacher", false),
      role("4", "guardian", false),
    ]);

    expect(options.map((option) => option.id)).toEqual([3, 4]);
  });
});
