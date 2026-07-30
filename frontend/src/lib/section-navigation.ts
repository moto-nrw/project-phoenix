/**
 * Shared catalogs for the grouped navigation sections (Datenverwaltung,
 * Eltern) plus the matching helpers.
 *
 * Vorher standen diese Listen in sidebar.tsx, und die Eltern-Pfade zusätzlich
 * nochmal in use-sidebar-accordion.ts — drei Kopien, die von Hand synchron
 * gehalten werden mussten. Die Breadcrumbs brauchen dieselben Labels, würden
 * also eine vierte Kopie anlegen. Deshalb liegt der Katalog hier: die
 * Seitenleiste, das Akkordeon-Auto-Aufklappen und die Breadcrumbs lesen alle
 * aus derselben Quelle, damit Seitenleisten-Eintrag und Breadcrumb nie
 * auseinanderlaufen.
 *
 * Die Planungsseiten haben ihren eigenen Katalog in planning-navigation.ts
 * (sie tragen zusätzlich Legacy-Redirect-Präfixe).
 */

export interface SectionSubPage {
  readonly href: string;
  readonly label: string;
}

/** Breadcrumb-Wurzel einer Sektion. `href` fehlt, wenn es keine Hub-Seite gibt. */
export interface SectionRoot {
  readonly label: string;
  readonly href?: string;
}

export const DATABASE_SECTION: SectionRoot = {
  label: "Datenverwaltung",
  href: "/database",
};

export const PARENT_SECTION: SectionRoot = {
  label: "Eltern",
  href: "/eltern",
};

/**
 * Planung hat keine eigene Hub-Seite: /planung ist nur ein Redirect-Frame auf
 * die erste Unterseite. Ohne `href` rendert die Breadcrumb den Namen als
 * reinen Text statt als Link ins Leere.
 */
export const PLANNING_SECTION: SectionRoot = {
  label: "Planung",
};

/** Unterseiten des Datenverwaltung-Akkordeons, in Anzeigereihenfolge. */
export const DATABASE_SUB_PAGES: readonly SectionSubPage[] = [
  { href: "/database/students", label: "Kinder" },
  { href: "/database/personal", label: "Personal" },
  { href: "/database/rooms", label: "Räume" },
  { href: "/database/activities", label: "Aktivitäten" },
  { href: "/database/groups", label: "Gruppen" },
  { href: "/database/roles", label: "Rollen" },
  { href: "/database/devices", label: "Geräte" },
  { href: "/database/permissions", label: "Berechtigungen" },
  { href: "/database/grade-transitions", label: "Jahrgangswechsel" },
  { href: "/database/exports", label: "Exporte" },
];

/**
 * Wählt die Sichtbarkeitsregel, die die Seitenleiste auf den Eintrag anwendet.
 * Die Regel selbst braucht Session und Settings und bleibt deshalb in
 * sidebar.tsx; hier steht nur, welche Regel gilt.
 */
export type ParentSubPageFeature =
  | "overview"
  | "messages"
  | "approvals"
  | "changeRequests"
  | "announcements"
  | "mealPlan";

export interface ParentSubPage extends SectionSubPage {
  readonly feature: ParentSubPageFeature;
}

/** Unterseiten des Eltern-Akkordeons, in Anzeigereihenfolge. */
export const PARENT_SUB_PAGES: readonly ParentSubPage[] = [
  // "Übersicht" (not "Überblick") so it does not collide with the Anmeldungen
  // accordion's "Überblick" hub — both would otherwise render simultaneously.
  { href: "/eltern", label: "Übersicht", feature: "overview" },
  { href: "/messages", label: "Nachrichten", feature: "messages" },
  {
    href: "/admin/guardian-approvals",
    label: "Konto-Anfragen",
    feature: "approvals",
  },
  {
    href: "/admin/change-requests",
    label: "Änderungsanfragen",
    feature: "changeRequests",
  },
  {
    href: "/parent-announcements",
    label: "Elternmitteilungen",
    feature: "announcements",
  },
  { href: "/meal-plan", label: "Essensplan", feature: "mealPlan" },
];

export function matchesSubPagePath(pathname: string, href: string): boolean {
  return pathname === href || pathname.startsWith(`${href}/`);
}

/** Längster passender Präfix gewinnt, damit z. B. /admin/change-requests nicht am kürzeren Eintrag hängenbleibt. */
function findActiveSubPage<T extends SectionSubPage>(
  pages: readonly T[],
  pathname: string,
): T | null {
  return (
    pages
      .filter((page) => matchesSubPagePath(pathname, page.href))
      .sort((a, b) => b.href.length - a.href.length)[0] ?? null
  );
}

export function getActiveDatabaseSubPage(
  pathname: string,
): SectionSubPage | null {
  return findActiveSubPage(DATABASE_SUB_PAGES, pathname);
}

export function getActiveParentSubPage(pathname: string): ParentSubPage | null {
  return findActiveSubPage(PARENT_SUB_PAGES, pathname);
}

export function getActiveParentSubPageHref(pathname: string): string | null {
  return getActiveParentSubPage(pathname)?.href ?? null;
}

/** Gehört der Pfad in den Eltern-Bereich (Hub oder eine Unterseite)? */
export function isElternPath(pathname: string): boolean {
  return getActiveParentSubPage(pathname) !== null;
}
