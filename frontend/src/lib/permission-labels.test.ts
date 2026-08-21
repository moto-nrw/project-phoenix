import { describe, it, expect } from "vitest";
import {
  resourceLabels,
  actionLabels,
  localizeResource,
  localizeAction,
  formatPermissionDisplay,
  localizeDescription,
} from "./permission-labels";

describe("permission-labels", () => {
  describe("resourceLabels", () => {
    it("should contain expected resource mappings", () => {
      expect(resourceLabels.users).toBe("Benutzer");
      expect(resourceLabels.roles).toBe("Rollen");
      expect(resourceLabels.permissions).toBe("Berechtigungen");
      expect(resourceLabels.activities).toBe("Aktivitäten");
      expect(resourceLabels.rooms).toBe("Räume");
      expect(resourceLabels.groups).toBe("Gruppen");
      expect(resourceLabels.visits).toBe("Besuche");
      expect(resourceLabels.substitutions).toBe("Vertretungen");
      expect(resourceLabels.schedules).toBe("Zeitpläne");
      expect(resourceLabels.config).toBe("Konfiguration");
      expect(resourceLabels.feedback).toBe("Feedback");
      expect(resourceLabels.iot).toBe("Geräte");
      expect(resourceLabels.system).toBe("System");
      expect(resourceLabels.admin).toBe("Administration");
      expect(resourceLabels.time_tracking).toBe("Zeiterfassung");
      expect(resourceLabels.grade_transitions).toBe("Klassenwechsel");
    });
  });

  describe("actionLabels", () => {
    it("should contain expected action mappings", () => {
      expect(actionLabels.create).toBe("Erstellen");
      expect(actionLabels.read).toBe("Lesen");
      expect(actionLabels.update).toBe("Bearbeiten");
      expect(actionLabels.delete).toBe("Löschen");
      expect(actionLabels.list).toBe("Auflisten");
      expect(actionLabels.manage).toBe("Verwalten");
      expect(actionLabels.assign).toBe("Zuweisen");
      expect(actionLabels.enroll).toBe("Einschreiben");
      expect(actionLabels.own).toBe("Eigene");
      expect(actionLabels.apply).toBe("Anwenden");
      expect(actionLabels["*"]).toBe("Alle");
    });
  });

  describe("localizeResource", () => {
    it("should return German label for known resources", () => {
      expect(localizeResource("users")).toBe("Benutzer");
      expect(localizeResource("rooms")).toBe("Räume");
      expect(localizeResource("activities")).toBe("Aktivitäten");
      expect(localizeResource("admin")).toBe("Administration");
    });

    it("should return original resource for unknown resources", () => {
      expect(localizeResource("unknown")).toBe("unknown");
      expect(localizeResource("custom_resource")).toBe("custom_resource");
      expect(localizeResource("something_else")).toBe("something_else");
    });

    it("should handle empty string", () => {
      expect(localizeResource("")).toBe("");
    });

    it("should handle special characters", () => {
      expect(localizeResource("special-resource")).toBe("special-resource");
      expect(localizeResource("resource_with_underscore")).toBe(
        "resource_with_underscore",
      );
    });
  });

  describe("localizeAction", () => {
    it("should return German label for known actions", () => {
      expect(localizeAction("create")).toBe("Erstellen");
      expect(localizeAction("read")).toBe("Lesen");
      expect(localizeAction("update")).toBe("Bearbeiten");
      expect(localizeAction("delete")).toBe("Löschen");
      expect(localizeAction("*")).toBe("Alle");
    });

    it("should return original action for unknown actions", () => {
      expect(localizeAction("unknown")).toBe("unknown");
      expect(localizeAction("custom_action")).toBe("custom_action");
      expect(localizeAction("something_else")).toBe("something_else");
    });

    it("should handle empty string", () => {
      expect(localizeAction("")).toBe("");
    });

    it("should handle special characters", () => {
      expect(localizeAction("special-action")).toBe("special-action");
      expect(localizeAction("action_with_underscore")).toBe(
        "action_with_underscore",
      );
    });
  });

  describe("formatPermissionDisplay", () => {
    it("should format known resource and action pairs", () => {
      expect(formatPermissionDisplay("users", "create")).toBe(
        "Benutzer: Erstellen",
      );
      expect(formatPermissionDisplay("rooms", "read")).toBe("Räume: Lesen");
      expect(formatPermissionDisplay("activities", "update")).toBe(
        "Aktivitäten: Bearbeiten",
      );
      expect(formatPermissionDisplay("groups", "delete")).toBe(
        "Gruppen: Löschen",
      );
    });

    it("should format with wildcard action", () => {
      expect(formatPermissionDisplay("admin", "*")).toBe(
        "Administration: Alle",
      );
      expect(formatPermissionDisplay("system", "*")).toBe("System: Alle");
    });

    it("should handle unknown resources with known actions", () => {
      expect(formatPermissionDisplay("unknown_resource", "create")).toBe(
        "unknown_resource: Erstellen",
      );
      expect(formatPermissionDisplay("custom", "read")).toBe("custom: Lesen");
    });

    it("should handle known resources with unknown actions", () => {
      expect(formatPermissionDisplay("users", "unknown_action")).toBe(
        "Benutzer: unknown_action",
      );
      expect(formatPermissionDisplay("rooms", "custom")).toBe("Räume: custom");
    });

    it("should handle both unknown resource and action", () => {
      expect(
        formatPermissionDisplay("unknown_resource", "unknown_action"),
      ).toBe("unknown_resource: unknown_action");
      expect(formatPermissionDisplay("custom_resource", "custom_action")).toBe(
        "custom_resource: custom_action",
      );
    });

    it("should handle empty strings", () => {
      expect(formatPermissionDisplay("", "")).toBe(": ");
      expect(formatPermissionDisplay("users", "")).toBe("Benutzer: ");
      expect(formatPermissionDisplay("", "create")).toBe(": Erstellen");
    });

    it("should handle all permutations correctly", () => {
      // All known
      expect(formatPermissionDisplay("users", "create")).toBe(
        "Benutzer: Erstellen",
      );
      // Known resource, unknown action
      expect(formatPermissionDisplay("users", "xyz")).toBe("Benutzer: xyz");
      // Unknown resource, known action
      expect(formatPermissionDisplay("xyz", "create")).toBe("xyz: Erstellen");
      // Both unknown
      expect(formatPermissionDisplay("xyz", "abc")).toBe("xyz: abc");
    });
  });

  describe("localizeDescription", () => {
    it("should return German description for known permissions", () => {
      expect(localizeDescription("users", "create")).toBe(
        "Neue Benutzer erstellen",
      );
      expect(localizeDescription("activities", "assign")).toBe(
        "Betreuer zu Aktivitäten zuweisen",
      );
      expect(localizeDescription("rooms", "manage")).toBe(
        "Raumverwaltung (Vollzugriff)",
      );
      expect(localizeDescription("time_tracking", "own")).toBe(
        "Eigene Arbeitszeiten erfassen",
      );
      expect(localizeDescription("grade_transitions", "apply")).toBe(
        "Klassenwechsel anwenden/zurücksetzen",
      );
    });

    it("should return German description for legacy permission names", () => {
      expect(localizeDescription("user", "create")).toBe(
        "Neue Benutzer erstellen",
      );
      expect(localizeDescription("role", "read")).toBe(
        "Rolleninformationen ansehen",
      );
      expect(localizeDescription("permission", "delete")).toBe(
        "Berechtigungen löschen",
      );
    });

    it("should fall back to DB description for unknown permissions", () => {
      expect(localizeDescription("unknown", "action", "DB description")).toBe(
        "DB description",
      );
    });

    it("should return empty string when no translation and no DB description", () => {
      expect(localizeDescription("unknown", "action")).toBe("");
      expect(localizeDescription("unknown", "action", undefined)).toBe("");
    });

    it("should prefer German translation over DB description", () => {
      expect(localizeDescription("users", "create", "Create new users")).toBe(
        "Neue Benutzer erstellen",
      );
    });

    it("should cover all major resource categories", () => {
      // Verify each resource category has at least one description
      expect(localizeDescription("substitutions", "manage")).toBe(
        "Vertretungsverwaltung (Vollzugriff)",
      );
      expect(localizeDescription("schedules", "create")).toBe(
        "Neue Stundenpläne erstellen",
      );
      expect(localizeDescription("visits", "read")).toBe("Besuche ansehen");
      expect(localizeDescription("feedback", "list")).toBe(
        "Feedback auflisten",
      );
      expect(localizeDescription("config", "update")).toBe(
        "Konfiguration bearbeiten",
      );
      expect(localizeDescription("iot", "manage")).toBe(
        "IoT-Geräteverwaltung (Vollzugriff)",
      );
      expect(localizeDescription("auth", "manage")).toBe(
        "Authentifizierungsverwaltung (Vollzugriff)",
      );
      expect(localizeDescription("system", "manage")).toBe(
        "Systemeinstellungen verwalten",
      );
      expect(localizeDescription("admin", "*")).toBe(
        "Vollzugriff auf alle Ressourcen",
      );
    });
  });
});
