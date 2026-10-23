import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import {
  BASE_ROLE_LABELS,
  getRoleDisplayDescription,
  getRoleDisplayName,
} from "./auth-helpers";
import { SYSTEM_ROLE_LABELS as ANNOUNCEMENT_ROLE_LABELS } from "./operator/announcements-helpers";
import {
  ROLE_TEMPLATES,
  SYSTEM_ROLE_LABELS,
  isHiddenSystemRole,
  isSystemRoleName,
  systemRoleDescription,
  systemRoleLabel,
  type SystemRoleName,
} from "./role-labels";

// One name per system role (#4574): every lookup the portals use reads the
// same source, and the backend's invitation mail and staff import name the
// roles the same way.

const EXPECTED: Record<SystemRoleName, string> = {
  admin: "Verwaltung",
  user: "Betreuung",
  lehrkraft: "Lehrkraft",
  guardian: "Eltern",
  guest: "Gast",
};

describe("role labels", () => {
  it("names each system role once", () => {
    for (const [role, name] of Object.entries(EXPECTED)) {
      expect(SYSTEM_ROLE_LABELS[role as SystemRoleName].name).toBe(name);
      expect(systemRoleLabel(role)).toBe(name);
      expect(getRoleDisplayName(role)).toBe(name);
    }
    for (const [role, name] of Object.entries(BASE_ROLE_LABELS)) {
      expect(name).toBe(EXPECTED[role as SystemRoleName]);
    }
    for (const [role, name] of Object.entries(ANNOUNCEMENT_ROLE_LABELS)) {
      expect(name).toBe(EXPECTED[role as SystemRoleName]);
    }
  });

  it.each(["constructor", "__proto__", "toString", "hasOwnProperty"])(
    "preserves the custom role %s",
    (name) => {
      expect(isSystemRoleName(name)).toBe(false);
      expect(systemRoleLabel(name)).toBeUndefined();
      expect(systemRoleDescription(name)).toBeUndefined();
      expect(getRoleDisplayName(name)).toBe(name);
      expect(getRoleDisplayDescription(name, "Eigene Beschreibung")).toBe(
        "Eigene Beschreibung",
      );
    },
  );

  it("hides only the system role Eltern from the role management", () => {
    expect(isHiddenSystemRole({ name: "guardian", isSystem: true })).toBe(true);
    expect(isHiddenSystemRole({ name: "Guardian", isSystem: true })).toBe(true);
    // A school's own role may carry the same name; it stays listed.
    expect(isHiddenSystemRole({ name: "guardian", isSystem: false })).toBe(
      false,
    );
    for (const name of ["admin", "user", "lehrkraft", "guest"]) {
      expect(isHiddenSystemRole({ name, isSystem: true })).toBe(false);
    }
  });

  it("names the templates after their role", () => {
    expect(ROLE_TEMPLATES.map((template) => template.label)).toEqual([
      "Wie Betreuung",
      "Wie Verwaltung",
      "Wie Lehrkraft",
    ]);
  });

  it("matches the names the backend sends in mails and accepts in imports", () => {
    // Vite rewrites the module URL, so resolve from the package root.
    const backend = resolve(process.cwd(), "../backend");
    const sources = [
      "modules/identityaccess/mails.go",
      "modules/dataimport/internal/application/importservice/staff_import_config.go",
    ];
    for (const source of sources) {
      const text = readFileSync(resolve(backend, source), "utf8");
      const block =
        /var systemRoleDisplayNames = map\[string\]string\{([^}]*)\}/.exec(
          text,
        )?.[1];
      expect(block, source).toBeDefined();
      const entries = Object.fromEntries(
        [...block!.matchAll(/"(\w+)":\s*"([^"]+)"/g)].map((m) => [m[1], m[2]]),
      );
      expect(entries, source).toEqual(EXPECTED);
    }
  });
});
