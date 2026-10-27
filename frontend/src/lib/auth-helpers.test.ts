import { describe, it, expect } from "vitest";
import {
  getAccountRoleLabel,
  getRoleDisplayName,
  getRoleDisplayDescription,
  isAssignableStaffRole,
  toAssignableRoleOptions,
  type Role,
} from "./auth-helpers";

describe("getRoleDisplayName", () => {
  // ADR 0091: „Extern“ ist keine Rolle mehr; eine gleichnamige Schulrolle
  // bleibt, wie sie gespeichert ist.
  it("keeps custom school roles as stored, extern included", () => {
    expect(getRoleDisplayName("extern")).toBe("extern");
    expect(getRoleDisplayName("externe-ag")).toBe("externe-ag");
  });

  // Die Demo-Schule legt beide Rollen klein geschrieben an
  // (identityaccess/internal/domain/demo_access.go).
  it("labels the demo school roles OGS-Leitung and Betreuungskraft", () => {
    expect(getRoleDisplayName("ogs-leitung")).toBe("OGS-Leitung");
    expect(getRoleDisplayName("betreuungskraft")).toBe("Betreuungskraft");
  });

  it("translates the system role ag-leitung to AG-Leitung", () => {
    expect(getRoleDisplayName("ag-leitung")).toBe("AG-Leitung");
    expect(getRoleDisplayName("AG-Leitung")).toBe("AG-Leitung");
  });

  it("translates admin to Verwaltung", () => {
    expect(getRoleDisplayName("admin")).toBe("Verwaltung");
  });

  it("translates Admin (case-insensitive) to Verwaltung", () => {
    expect(getRoleDisplayName("Admin")).toBe("Verwaltung");
  });

  it("translates ADMIN (uppercase) to Verwaltung", () => {
    expect(getRoleDisplayName("ADMIN")).toBe("Verwaltung");
  });

  it("translates user to Betreuung", () => {
    expect(getRoleDisplayName("user")).toBe("Betreuung");
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

  it("translates guardian to Eltern", () => {
    expect(getRoleDisplayName("guardian")).toBe("Eltern");
  });

  it("returns original name for non-system roles", () => {
    expect(getRoleDisplayName("custom_role")).toBe("custom_role");
  });
});

describe("getRoleDisplayDescription", () => {
  it("translates admin description to German", () => {
    expect(getRoleDisplayDescription("admin", "System administrator")).toBe(
      "Darf alles an der Schule einrichten und verwalten.",
    );
  });

  it("translates user description to Betreuung description", () => {
    expect(getRoleDisplayDescription("user", "Standard user")).toBe(
      "Betreut Kinder und Gruppen im Alltag.",
    );
  });

  // ADR 0091: Gast is a signed-in account that only reads.
  it("translates guest description to German", () => {
    expect(getRoleDisplayDescription("guest", "Limited access")).toBe(
      "Nur ansehen: Kinder, Räume und AGs. Keine Elterndaten, keine Nachrichten.",
    );
  });

  it("describes the system role ag-leitung", () => {
    expect(getRoleDisplayDescription("ag-leitung", "Supervises")).toBe(
      "Betreut eigene Termine: starten, Kinder ein- und auschecken, beenden",
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
      "Sieht die Daten der eigenen Kinder.",
    );
  });

  it("returns original description for non-system roles", () => {
    expect(getRoleDisplayDescription("custom", "Custom description")).toBe(
      "Custom description",
    );
  });

  it("handles case-insensitive role names", () => {
    expect(getRoleDisplayDescription("ADMIN", "Admin")).toBe(
      "Darf alles an der Schule einrichten und verwalten.",
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

  // ADR 0091: Gast is the view-only access, AG-Leitung supervises own
  // appointments; both are offered again.
  it("offers the system roles Gast and AG-Leitung", () => {
    const options = toAssignableRoleOptions([
      role("1", "guest", true),
      role("2", "user", true),
      role("3", "ag-leitung", true),
      role("4", "guardian", true),
    ]);

    expect(options.map((option) => option.name)).toEqual([
      "Gast",
      "Betreuung",
      "AG-Leitung",
    ]);
    expect(isAssignableStaffRole("guest", true)).toBe(true);
    expect(isAssignableStaffRole("guardian", true)).toBe(false);
  });

  it("excludes roles based on the guardian base role", () => {
    const options = toAssignableRoleOptions([
      { ...role("1", "Elternkoordination", false), baseRole: "guardian" },
      { ...role("2", "Betreuung", false), baseRole: "user" },
    ]);

    expect(options.map((option) => option.id)).toEqual(["2"]);
  });

  it("offers the system role ag-leitung as AG-Leitung in the invitation picker", () => {
    const options = toAssignableRoleOptions([
      { ...role("6", "ag-leitung", true), baseRole: "user" },
    ]);

    expect(options).toEqual([
      { id: "6", name: "AG-Leitung", systemName: "ag-leitung" },
    ]);
  });
});

describe("getAccountRoleLabel", () => {
  it("names the demo's school roles as the visitor chose them (#3891)", () => {
    expect(getAccountRoleLabel(["ogs-leitung"])).toBe("OGS-Leitung");
    expect(getAccountRoleLabel(["betreuungskraft"])).toBe("Betreuungskraft");
  });

  it("names an administrator Verwaltung", () => {
    expect(getAccountRoleLabel(["admin"])).toBe("Verwaltung");
    expect(getAccountRoleLabel(["ogs-leitung", "admin"])).toBe("Verwaltung");
  });

  it("prefers a school role over the system role behind it", () => {
    expect(getAccountRoleLabel(["user", "hausaufgabenhilfe"])).toBe(
      "Hausaufgabenhilfe",
    );
  });

  it("falls back to the system roles", () => {
    expect(getAccountRoleLabel(["user"])).toBe("Betreuung");
    expect(getAccountRoleLabel(["teacher"])).toBe("Betreuung");
    expect(getAccountRoleLabel([])).toBe("Betreuung");
    expect(getAccountRoleLabel(["guest"])).toBe("Gast");
  });

  // ADR 0091: the two access levels of external caregivers are system roles.
  it("names the system roles Gast and AG-Leitung", () => {
    expect(getAccountRoleLabel(["ag-leitung"])).toBe("AG-Leitung");
    expect(getAccountRoleLabel(["ag-leitung"], [true])).toBe("AG-Leitung");
    expect(getAccountRoleLabel(["guest"], [true])).toBe("Gast");
  });

  it("uses the role origin when a school role has a system role's name", () => {
    expect(getAccountRoleLabel(["user"], [false])).toBe("User");
    expect(getAccountRoleLabel(["user"], [true])).toBe("Betreuung");
    expect(getAccountRoleLabel(["user", "user"], [true, false])).toBe("User");
  });
});
