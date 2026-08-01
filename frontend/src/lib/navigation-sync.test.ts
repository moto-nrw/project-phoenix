import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

import {
  getPageTitle,
  getSectionBreadcrumb,
} from "~/components/dashboard/header/breadcrumb-utils";
import { PLANNING_SUB_PAGES } from "~/lib/planning-navigation";
import {
  DATABASE_SECTION,
  DATABASE_SUB_PAGES,
  ENROLLMENT_SECTION,
  ENROLLMENT_SUB_PAGES,
  PARENT_SECTION,
  PARENT_SUB_PAGES,
  PLANNING_SECTION,
  STAFF_FLAT_PAGES,
  type SectionRoot,
  type SectionSubPage,
} from "~/lib/section-navigation";

/**
 * Der Sync-Test zwischen Navigationskatalogen und Breadcrumbs.
 *
 * Das eigentliche Driftrisiko der Kopfzeile ist nicht der Katalog, sondern die
 * Prüfreihenfolge in getPageTitle(): eine neue Seite kann von einer früheren
 * Regel abgefangen werden (so wurde /staff/dienstplan mal als
 * "Mitarbeiter Details" gerendert) oder still auf "Home" fallen. Beides sieht
 * man im Code nicht, nur im Browser.
 *
 * Deshalb läuft hier jeder Katalogeintrag einmal durch die echten Header-
 * Helfer. Ein neuer Eintrag in einem Katalog ist damit automatisch abgedeckt —
 * wer eine Seite hinzufügt, muss diesen Test nicht anfassen, er bricht nur,
 * wenn Katalog und Kopfzeile auseinanderlaufen.
 */

const CATALOGS: readonly {
  readonly name: string;
  readonly root: SectionRoot;
  readonly pages: readonly SectionSubPage[];
}[] = [
  {
    name: "Datenverwaltung",
    root: DATABASE_SECTION,
    pages: DATABASE_SUB_PAGES,
  },
  { name: "Planung", root: PLANNING_SECTION, pages: PLANNING_SUB_PAGES },
  { name: "Eltern", root: PARENT_SECTION, pages: PARENT_SUB_PAGES },
  {
    name: "Anmeldungen",
    root: ENROLLMENT_SECTION,
    pages: ENROLLMENT_SUB_PAGES,
  },
];

describe("navigation catalogs stay in sync with the header", () => {
  describe.each(CATALOGS)("$name", ({ root, pages }) => {
    it.each(
      pages
        // Die Hub-Seite trägt in der Seitenleiste ein Unterpunkt-Label
        // ("Übersicht"), in der Kopfzeile aber ihren Sektionsnamen
        // ("Eltern") — bewusst unterschiedlich, siehe unten.
        .filter((page) => page.href !== root.href)
        .map((page) => [page.href, page.label]),
    )("titles %s as its catalog label", (href, label) => {
      expect(getPageTitle(href)).toBe(label);
    });

    it("titles the section hub with something other than the Home fallback", () => {
      if (!root.href) return; // Planung hat keine Hub-Seite.
      expect(getPageTitle(root.href)).not.toBe("Home");
    });

    it("has no duplicate hrefs", () => {
      const hrefs = pages.map((page) => page.href);
      expect(new Set(hrefs).size).toBe(hrefs.length);
    });

    it("keeps every sub-page href inside its own section or reachable on its own", () => {
      // Ein Katalogeintrag darf nie leer sein; ein "" oder "/" würde jeden
      // Pfad matchen und die ganze Sektion kapern.
      for (const page of pages) {
        expect(page.href.length).toBeGreaterThan(1);
        expect(page.href.startsWith("/")).toBe(true);
        expect(page.label.trim()).not.toBe("");
      }
      if (root.href) expect(root.href.startsWith("/")).toBe(true);
    });
  });

  // Anmeldungen rendert bewusst eine eigene Breadcrumb-Komponente
  // (EnrollmentBreadcrumb) und steht deshalb nicht in BREADCRUMB_SECTIONS.
  const SECTION_BREADCRUMB_CATALOGS = CATALOGS.filter(
    ({ root }) => root !== ENROLLMENT_SECTION,
  );

  describe.each(SECTION_BREADCRUMB_CATALOGS)(
    "$name breadcrumb",
    ({ root, pages }) => {
      it.each(
        pages
          // Die Hub-Seite zeigt nur ihren Sektionsnamen, keine Breadcrumb auf
          // sich selbst.
          .filter((page) => page.href !== root.href)
          .map((page) => [page.href, page.label]),
      )("builds a section breadcrumb for %s", (href, label) => {
        const crumb = getSectionBreadcrumb(href);
        expect(crumb).not.toBeNull();
        expect(crumb?.sectionLabel).toBe(root.label);
        expect(crumb?.sectionHref).toBe(root.href);
        expect(crumb?.pageLabel).toBe(label);
      });
    },
  );

  describe("flat staff pages", () => {
    it.each(
      Object.values(STAFF_FLAT_PAGES).map((page) => [page.href, page.label]),
    )("titles %s as its catalog label", (href, label) => {
      expect(getPageTitle(href)).toBe(label);
    });
  });

  /**
   * Seiten, die ihren Titel selbst per `useSetBreadcrumb({ pageTitle })`
   * setzen, überschreiben getPageTitle — aber erst in einem Effekt, also nach
   * dem ersten Frame. Weichen die beiden Werte ab, blitzt beim Laden kurz das
   * falsche Wort auf. Genau das war bei /operator/suggestions der Fall
   * ("Vorschläge" statt "Feedback").
   *
   * Der Test liest die Seiten direkt aus dem Quelltext, damit eine neue
   * Operator-Seite automatisch mitgeprüft wird.
   */
  describe("static page titles agree with the header fallback", () => {
    const APP_DIR = join(dirname(fileURLToPath(import.meta.url)), "..", "app");
    const SET_BREADCRUMB_TITLE =
      /useSetBreadcrumb\(\{\s*pageTitle:\s*"([^"]+)"\s*,?\s*\}\)/;

    function collectStaticTitles(dir: string): [string, string][] {
      const found: [string, string][] = [];
      for (const entry of readdirSync(dir)) {
        const full = join(dir, entry);
        if (statSync(full).isDirectory()) {
          found.push(...collectStaticTitles(full));
          continue;
        }
        if (entry !== "page.tsx") continue;
        const title = SET_BREADCRUMB_TITLE.exec(
          readFileSync(full, "utf8"),
        )?.[1];
        if (!title) continue;

        const route = full
          .slice(APP_DIR.length)
          .replace(/\/page\.tsx$/, "")
          .replace(/\/\([^)]+\)/g, "")
          .replace(/^\/\[tenant\]/, "");
        // Dynamische Segmente haben keinen festen Pfad, den getPageTitle
        // nachschlagen könnte.
        if (route.includes("[")) continue;
        found.push([route || "/", title]);
      }
      return found;
    }

    const staticTitles = collectStaticTitles(APP_DIR);

    it("finds the pages it is supposed to guard", () => {
      // Schutz gegen eine stillschweigend leere Prüfung, falls sich die
      // Schreibweise von useSetBreadcrumb einmal ändert.
      expect(staticTitles.length).toBeGreaterThanOrEqual(9);
    });

    it.each(staticTitles)(
      "%s renders '%s' on the first frame too",
      (route, title) => {
        expect(getPageTitle(route)).toBe(title);
      },
    );
  });

  it("never routes a catalog page to the 'Home' fallback", () => {
    // Die Regressionen, die dieser Test fängt, sehen im Code harmlos aus: eine
    // frühere Regel in getPageTitle() schluckt den Pfad, und die Kopfzeile
    // zeigt kommentarlos "Home".
    const homeTitled = CATALOGS.flatMap(({ pages }) => pages)
      .filter((page) => getPageTitle(page.href) === "Home")
      .map((page) => page.href);

    expect(homeTitled).toEqual([]);
  });
});
