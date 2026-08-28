import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

import {
  getPageTitle,
  getSectionBreadcrumb,
} from "~/components/dashboard/header/breadcrumb-utils";
import {
  additionalNavItems,
  ADMIN_MAIN_ITEMS,
  STAFF_MAIN_ITEMS,
} from "~/components/dashboard/mobile-bottom-nav";
import { PLANNING_SUB_PAGES } from "~/lib/planning-navigation";
import {
  ENROLLMENT_SECTION,
  ENROLLMENT_SUB_PAGES,
  PARENT_SECTION,
  PARENT_SUB_PAGES,
  PLANNING_SECTION,
  REPORTS_SECTION,
  REPORTS_SUB_PAGES,
  STAFF_FLAT_PAGES,
  TAB_PAGES,
  TEAM_SECTION,
  TEAM_SUB_PAGES,
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
  { name: "Planung", root: PLANNING_SECTION, pages: PLANNING_SUB_PAGES },
  { name: "Eltern", root: PARENT_SECTION, pages: PARENT_SUB_PAGES },
  { name: "Team", root: TEAM_SECTION, pages: TEAM_SUB_PAGES },
  { name: "Auswertung", root: REPORTS_SECTION, pages: REPORTS_SUB_PAGES },
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
   * falsche Wort auf.
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
      expect(staticTitles.length).toBeGreaterThanOrEqual(8);
    });

    it.each(staticTitles)(
      "%s renders '%s' on the first frame too",
      (route, title) => {
        expect(getPageTitle(route)).toBe(title);
      },
    );
  });

  /**
   * Ein Begriff, ein Wort. Die mobile Navigation kürzte einige Namen ab
   * („Suchen" statt „Kinder"), sodass dieselbe Fläche unten anders hieß als in
   * der Seitenleiste und in der Kopfzeile. Reicht der Platz nicht, ist der
   * Name zu lang — nicht die Leiste zu schmal.
   */
  describe("mobile navigation uses the same words as the header", () => {
    const mobileItems = [
      ...ADMIN_MAIN_ITEMS,
      ...STAFF_MAIN_ITEMS,
      ...additionalNavItems,
    ].filter((item) => !item.href.startsWith("/operator"));

    it("finds the entries it is supposed to guard", () => {
      expect(mobileItems.length).toBeGreaterThanOrEqual(15);
    });

    it.each(
      [...new Map(mobileItems.map((item) => [item.href, item])).values()].map(
        (item) => [item.href, item.label] as const,
      ),
    )("labels %s '%s' like the header does", (href, label) => {
      expect(getPageTitle(href)).toBe(label);
    });
  });

  /**
   * Die Register der aufgelösten Datenverwaltung sind Reiter an ihrer Fläche.
   * Die Brotkrume muss deshalb zuerst die Fläche nennen, dann den Reiter.
   */
  describe("register tabs hang under their collection", () => {
    it.each(
      TAB_PAGES.map((page) => [page.href, page.parent.label, page.label]),
    )("%s reads as '%s › %s'", (href, parentLabel, label) => {
      const crumb = getSectionBreadcrumb(href);
      expect(crumb).not.toBeNull();
      expect(crumb?.sectionLabel).toBe(parentLabel);
      expect(crumb?.pageLabel).toBe(label);
      expect(getPageTitle(href)).toBe(label);
    });

    it("has no duplicate hrefs", () => {
      const hrefs = TAB_PAGES.map((page) => page.href);
      expect(new Set(hrefs).size).toBe(hrefs.length);
    });
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
