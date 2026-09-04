/**
 * Shared catalogs for the navigation sections (Datenverwaltung, Eltern,
 * Team, Anmeldungen) plus the matching helpers.
 *
 * Vorher standen diese Listen in sidebar.tsx, und die Eltern- bzw.
 * Anmeldungs-Pfade zusätzlich nochmal in use-sidebar-accordion.ts — Kopien,
 * die von Hand synchron gehalten werden mussten. Die Breadcrumbs brauchen
 * dieselben Labels und hätten weitere Kopien angelegt. Deshalb liegt der
 * Katalog hier: die Seitenleiste, das mobile Mehr-Menü, das Auto-Aufklappen
 * und die Breadcrumbs lesen alle aus derselben Quelle, damit
 * Seitenleisten-Eintrag und Breadcrumb nie auseinanderlaufen.
 *
 * Die Planungsseiten haben ihren eigenen Katalog in planning-navigation.ts
 * (sie tragen zusätzlich Legacy-Redirect-Präfixe), benutzen aber denselben
 * `matchesPathPrefix` von hier.
 *
 * Welche Seite in welcher Gruppe der Seitenleiste steht und in welcher
 * Reihenfolge, sagt `staff-navigation.ts` (#2826). Hier steht nur, wie eine
 * Seite heißt, wo sie liegt und zu welcher Breadcrumb-Sektion sie gehört.
 *
 * NEUEN BEREICH ANLEGEN: Katalog (`*_SECTION` + `*_SUB_PAGES`) hier
 * ergänzen, `getActive*SubPage` daraus ableiten, in
 * `breadcrumb-utils.getSectionBreadcrumb` eine Zeile hinzufügen und die
 * Seiten in `staff-navigation.ts` einer Gruppe zuordnen. Die Tests
 * `navigation-sync.test.ts` und `staff-navigation.test.ts` prüfen danach
 * automatisch, dass jeder Eintrag einen Breadcrumb, einen Seitentitel und
 * genau einen Platz in der Seitenleiste bekommt.
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

export const DATABASE_SECTION: HubSectionRoot = {
  label: "Datenverwaltung",
  href: "/database",
};

/**
 * Eltern ist seit #2826 eine Gruppe der Seitenleiste, kein Akkordeon mit
 * Hub-Seite mehr: die frühere Übersicht /eltern war nur ein zweiter Weg zu
 * Seiten, die jetzt direkt in der Gruppe stehen. Ohne `href` rendert die
 * Breadcrumb den Namen als reinen Text, wie bei Planung.
 */
export const PARENT_SECTION: SectionRoot = {
  label: "Eltern",
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

/**
 * Team-intern: Team-Chat und Tagesinformationen. Keine Hub-Seite. In der
 * Seitenleiste stehen beide in der Gruppe „Team" neben Zeiterfassung, Mein
 * Kalender und Mitarbeiter (#2826); die Breadcrumb trägt denselben Namen,
 * damit Leiste und Kopfzeile dasselbe Wort zeigen. Bewusst getrennt von
 * Eltern: dort steht, was die Einrichtung mit den Familien austauscht, hier,
 * was intern bleibt.
 */
export const COMMUNICATION_SECTION: SectionRoot = {
  label: "Team",
};

/** Unterseiten des Datenverwaltung-Akkordeons, in Anzeigereihenfolge. */
export const DATABASE_SUB_PAGES: readonly SectionSubPage[] = [
  // „Kinderdaten", nicht „Kinder": die Seite ist der Datensatz (anlegen,
  // importieren, Betreuung beenden). Der laufende Tag heißt „Alle Kinder"
  // und steht im Tagesbetrieb; zwei Einträge namens „Kinder" waren genau die
  // Verwechslung aus #2826 (ADR 0008).
  { href: "/database/students", label: "Kinderdaten" },
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
type ParentSubPageFeature =
  "messages" | "approvals" | "announcements" | "bankDetails" | "mealPlan";

export interface ParentSubPage extends SectionSubPage {
  readonly feature: ParentSubPageFeature;
}

type CommunicationSubPageFeature = "teamChat" | "staffNotices";

export interface CommunicationSubPage extends SectionSubPage {
  readonly feature: CommunicationSubPageFeature;
}

/** Team-interne Seiten der Gruppe „Team", in Anzeigereihenfolge. */
export const COMMUNICATION_SUB_PAGES: readonly CommunicationSubPage[] = [
  // OGS-interner Team-Chat (#2598). Bewusst NICHT „Nachrichten": so heißt der
  // Eltern-Chat in der Gruppe Eltern. Zwei gleich benannte Einträge waren genau
  // der Grund, warum Schulen ihre Anfragen am falschen Ort gesucht haben.
  { href: "/team-chat", label: "Team-Chat", feature: "teamChat" },
  // Tagesinformationen (#2180): Hinweise der Leitung an das ganze Team.
  // Lesen alle, anlegen nur Admins — die Seite selbst trennt das.
  {
    href: "/tagesinformationen",
    label: "Tagesinformationen",
    feature: "staffNotices",
  },
];

/** Seiten der Gruppe „Eltern", in Anzeigereihenfolge. */
export const PARENT_SUB_PAGES: readonly ParentSubPage[] = [
  { href: "/messages", label: "Nachrichten", feature: "messages" },
  {
    // One entry, two jobs: the page itself splits Mitteilungen from Umfragen
    // (#1371). Both are the same broadcast workflow, so a second nav item would
    // only make staff guess which one they need.
    href: "/parent-announcements",
    label: "Mitteilungen",
    feature: "announcements",
  },
  // Die Elternanfragen sind in das Top-Level-Modul "Anfragen" umgezogen
  // (#2429); /admin/change-requests leitet dorthin um.
  {
    // „Elternzugänge", nicht „Konto-Anfragen": neben dem Modul „Anfragen"
    // wären das zwei Einträge mit demselben Wortstamm (#2826). Die Seite gibt
    // Zugänge zum Elternportal frei.
    href: "/admin/guardian-approvals",
    label: "Elternzugänge",
    feature: "approvals",
  },
  {
    href: "/eltern/bankverbindungen",
    label: "Bankverbindungen",
    feature: "bankDetails",
  },
  { href: "/meal-plan", label: "Essensplan", feature: "mealPlan" },
];

/**
 * Die flachen Seiten der Mitarbeiter-Navigation: alles, was in der
 * Seitenleiste als einzelner Eintrag steht und zu keinem Katalog-Bereich
 * gehört.
 *
 * Titel und Pfad standen bisher doppelt im Code — einmal in `NAV_ITEMS`
 * (sidebar.tsx), einmal in `mainRoutes` (breadcrumb-utils.ts). Wer eine Seite
 * umbenannte und nur eine Stelle traf, bekam eine Seitenleiste und eine
 * Kopfzeile mit unterschiedlichen Wörtern.
 *
 * Hier steht nur, wie die Seite heißt und wo sie liegt. Symbol, Farbe und
 * Sichtbarkeitsregeln bleiben in `NAV_ITEMS`, wo sie hingehören: sie sind
 * Darstellung, kein Navigationsfakt.
 *
 * Die mobile Navigation kürzt einige Namen bewusst ("Suchen" statt
 * "Alle Kinder", weil unter einem Symbol nur wenig Platz ist) und weicht
 * deshalb ab. Der Test in navigation-sync.test.ts prüft trotzdem, dass jeder
 * mobile Pfad einen Kopfzeilen-Titel bekommt.
 */
export const STAFF_FLAT_PAGES = {
  dashboard: { href: "/dashboard", label: "Home" },
  studentSearch: { href: "/students/search", label: "Alle Kinder" },
  activities: { href: "/activities", label: "Aktivitäten" },
  rooms: { href: "/rooms", label: "Räume" },
  staff: { href: "/staff", label: "Mitarbeiter" },
  // Anfragen-Modul (#2429): eingereichte Wünsche von Eltern und Mitarbeitenden
  // an einem Ort, mit Reitern nach Herkunft.
  anfragen: { href: "/anfragen", label: "Anfragen" },
  // Tages-Betreuungsplan (#2383): die Standardseite der Betreuungskräfte —
  // bewusst eine eigene Top-Level-Route, keine Unterseite des Admin-
  // Planungsbereichs /betreuungsplan.
  tagesplan: { href: "/tagesplan", label: "Tagesplan" },
  calendar: { href: "/calendar", label: "Mein Kalender" },
  // Dateiablage (#2596): gemeinsame Dateien der OGS mit Ordner-Freigaben.
  // Bewusst „Dateien“, nicht „Dokumente“: der Dokumente-Tab bei Kind und
  // Personal ist ein anderer Ort.
  dateien: { href: "/dateien", label: "Dateien" },
  substitutions: { href: "/substitutions", label: "Vertretungen" },
  infoDisplays: { href: "/info-displays", label: "Info-Displays" },
  timeTracking: { href: "/time-tracking", label: "Zeiterfassung" },
  dayLog: { href: "/day-log", label: "Tagesauswertung" },
  // Statistik (#2606): Quoten je Kind, Gruppe und Zeitraum plus Raumauslastung.
  statistics: { href: "/statistics", label: "Statistik" },
  emergency: { href: "/emergency", label: "Notfall" },
  help: { href: "/help", label: "Hilfe" },
  settings: { href: "/settings", label: "Einstellungen" },
} as const satisfies Record<string, SectionSubPage>;

/** Unterseiten des Anmeldungen-Akkordeons, in Anzeigereihenfolge. */
export const ENROLLMENT_SUB_PAGES: readonly SectionSubPage[] = [
  { href: "/admin/enrollments", label: "Überblick" },
  // Kein eigener Eintrag mehr für Änderungsanfragen: Anmeldungsänderungen
  // leben seit #2435 im Anfragen-Modul (/anfragen, Reiter „Eltern"). Zwei
  // gleichnamige Sidebar-Einträge waren genau der Grund, warum Schulen ihre
  // Anfragen am falschen Ort suchten.
  { href: "/enrollment-phases", label: "Anmeldephasen" },
  { href: "/care-offerings", label: "Betreuungsangebote" },
  { href: "/enrollment-form", label: "Anmeldeformulare" },
];

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

export function getActiveCommunicationSubPage(
  pathname: string,
): CommunicationSubPage | null {
  return findActiveSubPage(COMMUNICATION_SUB_PAGES, pathname);
}

export function getActiveCommunicationSubPageHref(
  pathname: string,
): string | null {
  return getActiveCommunicationSubPage(pathname)?.href ?? null;
}

/** Gehört der Pfad in den Kommunikation-Bereich (Team-Chat, Tagesinformationen)? */
export function isCommunicationPath(pathname: string): boolean {
  return getActiveCommunicationSubPage(pathname) !== null;
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

/** Gehört der Pfad in den Eltern-Bereich (Hub oder eine Unterseite)? */
export function isElternPath(pathname: string): boolean {
  return getActiveParentSubPage(pathname) !== null;
}

/** Gehört der Pfad in den Anmeldungen-Bereich (Hub oder eine Unterseite)? */
export function isEnrollmentPath(pathname: string): boolean {
  return getActiveEnrollmentSubPage(pathname) !== null;
}
