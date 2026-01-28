import { describe, it, expect } from "vitest";
import { getRoleDisplayName, getRoleDisplayDescription } from "./auth-helpers";

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

  it("translates user to Nutzer", () => {
    expect(getRoleDisplayName("user")).toBe("Nutzer");
  });

  it("translates guest to Gast", () => {
    expect(getRoleDisplayName("guest")).toBe("Gast");
  });

  it("returns original name for non-system roles", () => {
    expect(getRoleDisplayName("teacher")).toBe("teacher");
    expect(getRoleDisplayName("custom_role")).toBe("custom_role");
  });
});

describe("getRoleDisplayDescription", () => {
  it("translates admin description to German", () => {
    expect(getRoleDisplayDescription("admin", "System administrator")).toBe(
      "Systemadministrator mit vollem Zugriff",
    );
  });

  it("translates user description to German", () => {
    expect(getRoleDisplayDescription("user", "Standard user")).toBe(
      "Standardbenutzer mit grundlegenden Berechtigungen",
    );
  });

  it("translates guest description to German", () => {
    expect(getRoleDisplayDescription("guest", "Limited access")).toBe(
      "Eingeschränkter Zugriff für nicht authentifizierte Benutzer",
    );
  });

  it("returns original description for non-system roles", () => {
    expect(getRoleDisplayDescription("teacher", "Teaching staff")).toBe(
      "Teaching staff",
    );
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
