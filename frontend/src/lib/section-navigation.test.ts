import type { Session } from "next-auth";
import { describe, expect, it } from "vitest";

import {
  DATABASE_PAGE_PERMISSIONS,
  DATABASE_SUB_PAGES,
  databasePagePermissions,
  hasAnyDatabasePagePermission,
} from "./section-navigation";

function sessionWith(permissions: readonly string[]): Session {
  return {
    user: { id: "1", roles: ["ogs-leitung"], permissions: [...permissions] },
    expires: "2099-01-01",
  } as unknown as Session;
}

// Jede Datenverwaltungsseite trägt das Recht ihrer Route (#3469): eine Seite
// ohne Eintrag bliebe dem Rollennamen `admin` vorbehalten, und eine
// Leitungsrolle der Schule käme nie hinein.
describe("DATABASE_PAGE_PERMISSIONS", () => {
  it("names a permission for every page of the Datenverwaltung but the activities", () => {
    // Die Aktivitäten-Stammdaten bleiben dem Adminzuschnitt vorbehalten: ihr
    // Recht hält auch die Standard-Betreuerrolle (siehe Katalog-Kommentar).
    const withoutPermission = DATABASE_SUB_PAGES.filter(
      (page) => databasePagePermissions(page.href).length === 0,
    ).map((page) => page.href);
    expect(withoutPermission).toEqual(["/database/activities"]);
  });

  it("lists no page the navigation does not know", () => {
    const known = new Set(DATABASE_SUB_PAGES.map((page) => page.href));
    for (const href of Object.keys(DATABASE_PAGE_PERMISSIONS)) {
      expect(known.has(href), `${href} is not a Datenverwaltung page`).toBe(
        true,
      );
    }
  });

  it("keeps the master-data pages behind the manage right, not the read right", () => {
    // Mit users:read allein sähe jede Betreuungskraft eine Kinderdaten-Seite,
    // deren Anlegen und Löschen ihr das Backend verweigert.
    const caregiver = sessionWith(["users:read", "rooms:read", "groups:read"]);
    expect(hasAnyDatabasePagePermission(caregiver, "/database/students")).toBe(
      false,
    );
    expect(hasAnyDatabasePagePermission(caregiver, "/database/rooms")).toBe(
      false,
    );
    expect(hasAnyDatabasePagePermission(caregiver, "/database/groups")).toBe(
      false,
    );
  });

  it("opens a page for any of its permissions", () => {
    expect(
      hasAnyDatabasePagePermission(
        sessionWith(["staff:stammdaten"]),
        "/database/personal",
      ),
    ).toBe(true);
    expect(
      hasAnyDatabasePagePermission(
        sessionWith(["users:manage"]),
        "/database/students",
      ),
    ).toBe(true);
    expect(
      hasAnyDatabasePagePermission(sessionWith(["users:manage"]), "/unknown"),
    ).toBe(false);
  });
});
