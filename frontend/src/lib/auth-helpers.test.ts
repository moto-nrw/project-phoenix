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

  it("translates teacher to Lehrkraft", () => {
    expect(getRoleDisplayName("teacher")).toBe("Lehrkraft");
  });

  it("translates staff to Betreuer", () => {
    expect(getRoleDisplayName("staff")).toBe("Betreuer");
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

  it("translates teacher description to German", () => {
    expect(getRoleDisplayDescription("teacher", "Teaching staff")).toBe(
      "Lehrkraft mit Gruppenmanagement-Berechtigungen",
    );
  });

  it("translates staff description to German", () => {
    expect(getRoleDisplayDescription("staff", "General staff")).toBe(
      "Pädagogische Fachkraft mit Betreuungsrechten",
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
