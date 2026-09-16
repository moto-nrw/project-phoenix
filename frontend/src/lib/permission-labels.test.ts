import { readFileSync } from "node:fs";
import { resolve } from "node:path";
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

    it("prefers a resource-specific label over the shared one", () => {
      // A school holds bank details of a parent, never tax data.
      expect(localizeAction("financial", "guardians")).toBe("Bankdaten");
      expect(localizeAction("financial", "staff")).toBe("Bank- & Steuerdaten");
      expect(localizeAction("financial")).toBe("Bank- & Steuerdaten");
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

  // The Berechtigungen list renders every row of auth.permissions through the
  // three tables above. Each lookup falls back silently — to the raw key, or to
  // the English description the migration wrote — so a permission added without
  // German wording reached the OGS-Leitung as "Aktivitäten: manage_categories /
  // Manage activity categories (school Stammdaten)" (#3238).
  //
  // The catalog is the backend's, not a copy: a migration that adds a
  // permission without extending catalog.json fails
  // TestPermissionCatalogMatchesMigratedDatabase, and extending catalog.json
  // without German wording fails here.
  describe("catalog coverage", () => {
    // Resolved from the Vitest root (frontend/), not from import.meta.url:
    // Vite rewrites the module URL to an http:// one, which fileURLToPath
    // rejects.
    const catalogPath = resolve(
      process.cwd(),
      "../backend/auth/authorize/permissions/catalog.json",
    );
    const catalog = JSON.parse(readFileSync(catalogPath, "utf8")) as {
      resource: string;
      action: string;
    }[];

    it("reads a non-empty backend catalog", () => {
      expect(catalog.length).toBeGreaterThan(0);
    });

    it.each(
      catalog.map((entry) => [`${entry.resource}:${entry.action}`, entry]),
    )("%s has German resource, action and description", (name, entry) => {
      expect(
        resourceLabels[entry.resource],
        `resourceLabels is missing "${entry.resource}" — ${name} would show the raw key`,
      ).toBeTruthy();
      expect(
        actionLabels[entry.action],
        `actionLabels is missing "${entry.action}" — ${name} would show the raw key`,
      ).toBeTruthy();

      // A German description is the only way localizeDescription can ignore
      // the database text, so asking for the sentinel back is exactly the
      // "falls through to English" case.
      const sentinel = "__db_description__";
      expect(
        localizeDescription(entry.resource, entry.action, sentinel),
        `permissionDescriptions is missing "${name}" — the list would show the English database text`,
      ).not.toBe(sentinel);
    });
  });
});
