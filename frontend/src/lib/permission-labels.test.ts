import { describe, it, expect } from "vitest";
import {
  resourceLabels,
  actionLabels,
  localizeResource,
  localizeAction,
  formatPermissionDisplay,
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
      expect(resourceLabels.schedules).toBe("Zeitpläne");
      expect(resourceLabels.config).toBe("Konfiguration");
      expect(resourceLabels.feedback).toBe("Feedback");
      expect(resourceLabels.iot).toBe("Geräte");
      expect(resourceLabels.system).toBe("System");
      expect(resourceLabels.admin).toBe("Administration");
    });
  });

  describe("actionLabels", () => {
    it("should contain expected action mappings", () => {
      expect(actionLabels.create).toBe("Erstellen");
      expect(actionLabels.read).toBe("Ansehen");
      expect(actionLabels.update).toBe("Bearbeiten");
      expect(actionLabels.delete).toBe("Löschen");
      expect(actionLabels.list).toBe("Auflisten");
      expect(actionLabels.manage).toBe("Verwalten");
      expect(actionLabels.assign).toBe("Zuweisen");
      expect(actionLabels.enroll).toBe("Anmelden");
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
      expect(localizeAction("read")).toBe("Ansehen");
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
      expect(formatPermissionDisplay("rooms", "read")).toBe("Räume: Ansehen");
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
      expect(formatPermissionDisplay("custom", "read")).toBe("custom: Ansehen");
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
});
