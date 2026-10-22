import { describe, it, expect } from "vitest";
import {
  getAccountRoleLabel,
  getRoleDisplayName,
  getRoleDisplayDescription,
  isExternalCaregiverRole,
  toAssignableRoleOptions,
  type Role,
} from "./auth-helpers";

describe("isExternalCaregiverRole", () => {
  // #4573: the backend finds the role by name, case-insensitive.
  it("recognizes the role Extern in any spelling, and nothing else", () => {
    expect(isExternalCaregiverRole("extern")).toBe(true);
    expect(isExternalCaregiverRole(" Extern ")).toBe(true);
    expect(isExternalCaregiverRole("externe-ag")).toBe(false);
    expect(isExternalCaregiverRole(undefined)).toBe(false);
  });
});

describe("getRoleDisplayName", () => {
  // #4573: die Schulrolle für externe Kräfte ist klein gespeichert.
  it("shows the provisioned school role extern as Extern", () => {
    expect(getRoleDisplayName("extern")).toBe("Extern");
    expect(getRoleDisplayName(" EXTERN ")).toBe("Extern");
  });

  it("keeps other custom school roles as stored", () => {
    expect(getRoleDisplayName("externe-ag")).toBe("externe-ag");
  });

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

    expect(options.map((option) => option.id)).toEqual(["3", "4"]);
  });

  it("excludes roles based on the guardian base role", () => {
    const options = toAssignableRoleOptions([
      { ...role("1", "Elternkoordination", false), baseRole: "guardian" },
      { ...role("2", "Betreuung", false), baseRole: "user" },
    ]);

    expect(options.map((option) => option.id)).toEqual(["2"]);
  });

  it("offers the school role extern as Extern in the invitation picker (#4573)", () => {
    const options = toAssignableRoleOptions([
      { ...role("5", "extern", false), baseRole: "user" },
    ]);

    expect(options).toEqual([
      { id: "5", name: "Extern", systemName: "extern" },
    ]);
  });
});

describe("getAccountRoleLabel", () => {
  it("names the demo's school roles as the visitor chose them (#3891)", () => {
    expect(getAccountRoleLabel(["ogs-leitung"])).toBe("OGS-Leitung");
    expect(getAccountRoleLabel(["betreuungskraft"])).toBe("Betreuungskraft");
  });

  it("keeps Admin for an administrator", () => {
    expect(getAccountRoleLabel(["admin"])).toBe("Admin");
    expect(getAccountRoleLabel(["ogs-leitung", "admin"])).toBe("Admin");
  });

  it("prefers a school role over the system role behind it", () => {
    expect(getAccountRoleLabel(["user", "hausaufgabenhilfe"])).toBe(
      "Hausaufgabenhilfe",
    );
  });

  it("falls back to the system roles", () => {
    expect(getAccountRoleLabel(["user"])).toBe("Betreuer");
    expect(getAccountRoleLabel(["teacher"])).toBe("Betreuer");
    expect(getAccountRoleLabel([])).toBe("Betreuer");
    expect(getAccountRoleLabel(["guest"])).toBe("Gast");
  });

  it("uses the role origin when a school role has a system role's name", () => {
    expect(getAccountRoleLabel(["user"], [false])).toBe("User");
    expect(getAccountRoleLabel(["user"], [true])).toBe("Betreuer");
    expect(getAccountRoleLabel(["user", "user"], [true, false])).toBe("User");
  });
});
