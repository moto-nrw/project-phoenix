/**
 * Shared catalogs for the grouped navigation sections (Eltern, Team,
 * Auswertung) plus the matching helpers.
 *
 * Vorher standen diese Listen in sidebar.tsx, und die Eltern- bzw.
 * Anmeldungs-Pfade zusätzlich nochmal in use-sidebar-accordion.ts — Kopien,
 * die von Hand synchron gehalten werden mussten. Die Breadcrumbs brauchen
 * dieselben Labels und hätten weitere Kopien angelegt. Deshalb liegt der
 * Katalog hier: die Seitenleiste, das Akkordeon-Auto-Aufklappen und die
 * Breadcrumbs lesen alle aus derselben Quelle, damit Seitenleisten-Eintrag
 * und Breadcrumb nie auseinanderlaufen.
 *
 * Die Planungsseiten haben ihren eigenen Katalog in planning-navigation.ts
 * (sie tragen zusätzlich Legacy-Redirect-Präfixe), benutzen aber denselben
 * `matchesPathPrefix` von hier.
 *
 * Der Bereich „Datenverwaltung" ist aufgelöst (BAUARTEN-SPEC, Teil 2): die
 * Register sind keine zweite Baumhälfte mehr, sondern Reiter an ihrem Objekt
 * (Kinder, Mitarbeitende, Räume, Aktivitäten) beziehungsweise in den
 * Einstellungen. Die Routen bleiben unverändert bestehen — nur der Weg
 * dorthin ist ein anderer. `RECORD_TABS` und `SETTINGS_REGISTER_TABS` sind
 * die eine Quelle für diese Reiter.
 *
 * NEUEN BEREICH ANLEGEN: Katalog (`*_SECTION` + `*_SUB_PAGES`) hier
 * ergänzen, `getActive*SubPage` daraus ableiten, und in
 * `breadcrumb-utils.BREADCRUMB_SECTIONS` eine Zeile hinzufügen. Der Test
 * `navigation-sync.test.ts` prüft danach automatisch, dass jeder Eintrag
 * einen Breadcrumb und einen Seitentitel bekommt.
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

/** Sektion mit Hub-Seite — `href` ist garantiert gesetzt. */
export interface HubSectionRoot extends SectionRoot {
  readonly href: string;
}

/**
 * Die Datenverwaltung ist als Navigationsbereich verschwunden, ihre Hub-Seite
 * bleibt aber erreichbar (Lesezeichen, Hilfe-Anleitung). Der Eintrag liefert
 * ihr weiterhin einen Kopfzeilen-Titel.
 */
export const DATABASE_SECTION: HubSectionRoot = {
  label: "Stammdaten",
  href: "/database",
};

export const PARENT_SECTION: HubSectionRoot = {
  label: "Eltern",
  href: "/eltern",
};

export const ENROLLMENT_SECTION: HubSectionRoot = {
  label: "Anmeldungen",
  href: "/admin/enrollments",
};

/**
 * Planung hat keine eigene Hub-Seite: /planung ist nur ein Redirect-Frame auf
 * die erste Unterseite. Ohne `href` rendert die Breadcrumb den Namen als
 * reinen Text statt als Link ins Leere.
 */
export const PLANNING_SECTION: SectionRoot = {
  label: "Planung",
};

/** Team (#2598/#2596): die internen Flächen der OGS. Keine Hub-Seite. */
export const TEAM_SECTION: SectionRoot = {
  label: "Team",
};

/** Auswertung: alles, was Zahlen über einen Zeitraum zeigt. Keine Hub-Seite. */
export const REPORTS_SECTION: SectionRoot = {
  label: "Auswertung",
};

/**
 * Wählt die Sichtbarkeitsregel, die die Seitenleiste auf den Eintrag anwendet.
 * Die Regel selbst braucht Session und Settings und bleibt deshalb in
 * sidebar.tsx; hier steht nur, welche Regel gilt.
 */
type ParentSubPageFeature =
  | "overview"
  | "messages"
  | "requests"
  | "approvals"
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
  // Anfragen-Modul (#2429): eingereichte Wünsche von Eltern und
  // Mitarbeitenden an einem Ort. Stand bis zum Navigationsumbau als eigener
  // Bereich neben „Konto-Anfragen" — zwei Namen mit demselben Wortstamm.
  // Jetzt steht es hier, und der Kontenzugang heißt „Neue Elternkonten".
  { href: "/anfragen", label: "Anfragen", feature: "requests" },
  {
    href: "/admin/guardian-approvals",
    label: "Neue Elternkonten",
    feature: "approvals",
  },
  {
    // One entry, two jobs: the page itself splits Mitteilungen from Umfragen
    // (#1371). Both are the same broadcast workflow, so a second nav item would
    // only make staff guess which one they need.
    href: "/parent-announcements",
    label: "Mitteilungen und Umfragen",
    feature: "announcements",
  },
  { href: "/meal-plan", label: "Essensplan", feature: "mealPlan" },
];

/**
 * Die flachen Seiten der Mitarbeiter-Navigation: alles, was in der
 * Seitenleiste als einzelner Eintrag steht statt in einem Akkordeon.
 *
 * Titel und Pfad standen bisher doppelt im Code — einmal in `NAV_ITEMS`
 * (sidebar.tsx), einmal in `mainRoutes` (breadcrumb-utils.ts). Wer eine Seite
 * umbenannte und nur eine Stelle traf, bekam eine Seitenleiste und eine
 * Kopfzeile mit unterschiedlichen Wörtern.
 *
 * Hier steht nur, wie die Seite heißt und wo sie liegt. Symbol und
 * Sichtbarkeitsregeln bleiben in `NAV_ITEMS`, wo sie hingehören: sie sind
 * Darstellung, kein Navigationsfakt.
 *
 * Ein Begriff, ein Wort: die mobile Navigation trägt genau diese Namen, sie
 * kürzt nicht mehr ab („Suchen" statt „Kinder"). Reicht der Platz nicht, ist
 * der Name zu lang — nicht die Leiste zu schmal. `navigation-sync.test.ts`
 * prüft das.
 */
export const STAFF_FLAT_PAGES = {
  dashboard: { href: "/dashboard", label: "Start" },
  studentSearch: { href: "/students/search", label: "Kinder" },
  activities: { href: "/activities", label: "Aktivitäten" },
  rooms: { href: "/rooms", label: "Räume" },
  staff: { href: "/staff", label: "Mitarbeitende" },
  // OGS-interner Team-Chat (#2598). Bewusst NICHT „Nachrichten": so heißt der
  // Eltern-Chat im Eltern-Akkordeon. Zwei gleich benannte Einträge waren genau
  // der Grund, warum Schulen ihre Anfragen am falschen Ort gesucht haben.
  teamChat: { href: "/team-chat", label: "Team-Chat" },
  anfragen: { href: "/anfragen", label: "Anfragen" },
  calendar: { href: "/calendar", label: "Kalender" },
  // Dateiablage (#2596): gemeinsame Dateien der OGS mit Ordner-Freigaben.
  // Bewusst „Dateien“, nicht „Dokumente“: der Dokumente-Tab bei Kind und
  // Personal ist ein anderer Ort.
  dateien: { href: "/dateien", label: "Dateien" },
  substitutions: { href: "/substitutions", label: "Vertretungszugriff" },
  infoDisplays: { href: "/info-displays", label: "Info-Displays" },
  timeTracking: { href: "/time-tracking", label: "Zeiterfassung" },
  dayLog: { href: "/day-log", label: "Tagesbericht" },
  // Statistik (#2606): Quoten je Kind, Gruppe und Zeitraum plus Raumauslastung.
  statistics: { href: "/statistics", label: "Statistik" },
  payroll: { href: "/payroll", label: "Abrechnung" },
  emergency: { href: "/emergency", label: "Notfall" },
  help: { href: "/help", label: "Hilfe" },
  settings: { href: "/settings", label: "Einstellungen" },
} as const satisfies Record<string, SectionSubPage>;

/** Unterseiten des Team-Bereichs, in Anzeigereihenfolge. */
export const TEAM_SUB_PAGES: readonly SectionSubPage[] = [
  STAFF_FLAT_PAGES.teamChat,
  STAFF_FLAT_PAGES.dateien,
];

/** Unterseiten des Auswertungs-Bereichs, in Anzeigereihenfolge. */
export const REPORTS_SUB_PAGES: readonly SectionSubPage[] = [
  STAFF_FLAT_PAGES.statistics,
  STAFF_FLAT_PAGES.timeTracking,
  STAFF_FLAT_PAGES.payroll,
  STAFF_FLAT_PAGES.dayLog,
];

/** Unterseiten des Anmeldungen-Blocks im Eltern-Bereich, in Anzeigereihenfolge. */
export const ENROLLMENT_SUB_PAGES: readonly SectionSubPage[] = [
  { href: "/admin/enrollments", label: ENROLLMENT_SECTION.label },
  { href: "/enrollment-phases", label: "Anmeldephasen" },
  // „Betreuungsangebote" hieß fast wie „Betreuungszeiten" und wie
  // „Betreuungsplan". Der Wortstamm „Betreuungs-" trägt in der Navigation
  // nur noch einen Namen: den Betreuungsplan.
  { href: "/care-offerings", label: "Angebote" },
  { href: "/enrollment-form", label: "Anmeldeformulare" },
];

/**
 * Ein Register der alten Datenverwaltung, das jetzt als Reiter an seinem
 * Objekt hängt. `parent` ist die Fläche, an der der Reiter steht — sie ist
 * zugleich die erste Stufe der Brotkrume.
 */
export interface TabPage extends SectionSubPage {
  readonly parent: SectionSubPage;
}

/**
 * Die Register an ihren Sammlungen. Die Seiten selbst bleiben liegen, wo sie
 * liegen; hier steht nur, wo man sie erreicht und wie der Reiter heißt.
 *
 * Die Reihenfolge ist die Anzeigereihenfolge der Reiter je Fläche.
 */
export const RECORD_TABS: readonly TabPage[] = [
  {
    href: "/database/students",
    label: "Stammdaten",
    parent: STAFF_FLAT_PAGES.studentSearch,
  },
  {
    href: "/database/personal",
    label: "Stammdaten",
    parent: STAFF_FLAT_PAGES.staff,
  },
  {
    // Alt-Seite mit eigenem Datenmodell (education.group_substitution):
    // vergibt temporären Gruppen-Datenzugriff, keine Personalplanung. Hieß
    // „Gruppenzugriff" und steht jetzt als Reiter bei den Mitarbeitenden.
    href: "/substitutions",
    label: STAFF_FLAT_PAGES.substitutions.label,
    parent: STAFF_FLAT_PAGES.staff,
  },
  {
    href: "/database/rooms",
    label: "Stammdaten",
    parent: STAFF_FLAT_PAGES.rooms,
  },
  {
    href: "/database/activities",
    label: "Stammdaten",
    parent: STAFF_FLAT_PAGES.activities,
  },
];

/**
 * Die Register, die reine Konfiguration sind und kein Tagesgeschäft: sie
 * stehen als Reiter in den Einstellungen, hinter den Schema-Reitern.
 */
export const SETTINGS_REGISTER_TABS: readonly TabPage[] = [
  {
    href: "/database/groups",
    label: "Gruppen",
    parent: STAFF_FLAT_PAGES.settings,
  },
  {
    href: "/database/roles",
    label: "Rollen",
    parent: STAFF_FLAT_PAGES.settings,
  },
  {
    href: "/database/permissions",
    label: "Berechtigungen",
    parent: STAFF_FLAT_PAGES.settings,
  },
  {
    href: "/database/devices",
    label: "Geräte",
    parent: STAFF_FLAT_PAGES.settings,
  },
  {
    href: "/info-displays",
    label: STAFF_FLAT_PAGES.infoDisplays.label,
    parent: STAFF_FLAT_PAGES.settings,
  },
  {
    href: "/database/exports",
    label: "Exporte",
    parent: STAFF_FLAT_PAGES.settings,
  },
  {
    href: "/database/grade-transitions",
    label: "Jahrgangswechsel",
    parent: STAFF_FLAT_PAGES.settings,
  },
];

/**
 * Alle Reiter-Seiten zusammen — die Quelle für Kopfzeilen-Titel und
 * Breadcrumbs dieser Seiten.
 */
export const TAB_PAGES: readonly TabPage[] = [
  ...RECORD_TABS,
  ...SETTINGS_REGISTER_TABS,
];

/** Reiter einer Fläche, in Anzeigereihenfolge. */
export function getTabsForCollection(collectionHref: string): TabPage[] {
  return TAB_PAGES.filter((page) => page.parent.href === collectionHref);
}

/**
 * Gehört `pathname` zu `prefix` — als exakter Treffer oder als Unterseite?
 * Die gemeinsame Pfadregel aller Navigationskataloge; ein blankes
 * `startsWith` würde z. B. /enrollment-phases-alt fälschlich mitzählen.
 */
export function matchesPathPrefix(pathname: string, prefix: string): boolean {
  // Das Zeichen hinter dem Präfix einzeln zu prüfen ist gleichbedeutend mit
  // startsWith(`${prefix}/`), legt dabei aber keinen String an — diese
  // Funktion ist die innere Schleife jeder Katalogsuche.
  return (
    pathname === prefix ||
    (pathname.startsWith(prefix) && pathname[prefix.length] === "/")
  );
}

/** Längster passender Präfix gewinnt, damit z. B. /admin/change-requests nicht am kürzeren Eintrag hängenbleibt. */
function findActiveSubPage<T extends SectionSubPage>(
  pages: readonly T[],
  pathname: string,
): T | null {
  let best: T | null = null;
  for (const page of pages) {
    if (!matchesPathPrefix(pathname, page.href)) continue;
    if (!best || page.href.length > best.href.length) best = page;
  }
  return best;
}

export function getActiveTabPage(pathname: string): TabPage | null {
  return findActiveSubPage(TAB_PAGES, pathname);
}

export function getActiveParentSubPage(pathname: string): ParentSubPage | null {
  return findActiveSubPage(PARENT_SUB_PAGES, pathname);
}

export function getActiveParentSubPageHref(pathname: string): string | null {
  return getActiveParentSubPage(pathname)?.href ?? null;
}

export function getActiveTeamSubPage(pathname: string): SectionSubPage | null {
  return findActiveSubPage(TEAM_SUB_PAGES, pathname);
}

export function getActiveTeamSubPageHref(pathname: string): string | null {
  return getActiveTeamSubPage(pathname)?.href ?? null;
}

export function getActiveReportsSubPage(
  pathname: string,
): SectionSubPage | null {
  return findActiveSubPage(REPORTS_SUB_PAGES, pathname);
}

export function getActiveReportsSubPageHref(pathname: string): string | null {
  return getActiveReportsSubPage(pathname)?.href ?? null;
}

export function getActiveEnrollmentSubPage(
  pathname: string,
): SectionSubPage | null {
  return findActiveSubPage(ENROLLMENT_SUB_PAGES, pathname);
}

export function getActiveEnrollmentSubPageHref(
  pathname: string,
): string | null {
  return getActiveEnrollmentSubPage(pathname)?.href ?? null;
}

/** Gehört der Pfad in den Eltern-Bereich (Hub, Unterseite oder Anmeldungen)? */
export function isElternPath(pathname: string): boolean {
  return (
    getActiveParentSubPage(pathname) !== null ||
    getActiveEnrollmentSubPage(pathname) !== null
  );
}

/** Gehört der Pfad in den Anmeldungen-Block? */
export function isEnrollmentPath(pathname: string): boolean {
  return getActiveEnrollmentSubPage(pathname) !== null;
}

/** Gehört der Pfad in den Team-Bereich? */
export function isTeamPath(pathname: string): boolean {
  return getActiveTeamSubPage(pathname) !== null;
}

/** Gehört der Pfad in den Auswertungs-Bereich? */
export function isReportsPath(pathname: string): boolean {
  return getActiveReportsSubPage(pathname) !== null;
}
