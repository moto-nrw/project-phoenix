import type { Icon as PhosphorIcon } from "@phosphor-icons/react";
import {
  BuildingOfficeIcon,
  CalendarBlankIcon,
  SunIcon,
  UsersFourIcon,
  UsersIcon,
} from "@phosphor-icons/react/ssr";

import type { MotoConceptKey } from "~/lib/moto-concepts";
import {
  getActivePlanningSubPage,
  PLANNING_SUB_PAGES,
} from "~/lib/planning-navigation";
import {
  COMMUNICATION_SUB_PAGES,
  ENROLLMENT_SUB_PAGES,
  matchesPathPrefix,
  PARENT_SUB_PAGES,
  STAFF_FLAT_PAGES,
} from "~/lib/section-navigation";

/**
 * Der Baum der Mitarbeiter-Navigation (#2826): welche Seite in welcher
 * Gruppe steht und in welcher Reihenfolge. Seitenleiste und mobiles
 * Mehr-Menü rendern beide aus dieser einen Liste; vorher hatte jede ihre
 * eigene Reihenfolge, und drei Seiten fehlten mobil ganz.
 *
 * Fünf Gruppen, benannt nach dem, was die Person gerade tut, nicht nach der
 * Technik dahinter:
 *
 * - **Tagesbetrieb** — mehrmals am Tag: die eigenen Gruppen, die laufende
 *   Aufsicht, alle Kinder, Räume, Aktivitäten, Vertretungen und die offenen
 *   Anfragen. Für alle Rollen beim ersten Besuch geöffnet.
 * - **Eltern** — alles, was die Einrichtung mit den Familien austauscht,
 *   plus die Anmeldungen.
 * - **Team** — die eigene Arbeitszeit, der eigene Kalender, die Kollegen und
 *   die interne Kommunikation.
 * - **Planung** — die Pläne der Einrichtung; sichtbar nur, wenn die Person
 *   mindestens eine Planungsseite öffnen darf.
 * - **Verwaltung** — Stammdaten, Auswertungen, Dateien, Info-Displays.
 *
 * Innerhalb einer Gruppe steht oben, was im OGS-Alltag am häufigsten
 * gebraucht wird. Die Einschätzung stammt aus den Arbeitsabläufen, nicht aus
 * Nutzungsdaten; sobald PostHog-Zahlen vorliegen, gehört die Reihenfolge
 * daran geprüft.
 *
 * Hier stehen nur Ordnung und Zugehörigkeit. Namen und Pfade kommen aus den
 * Katalogen (`section-navigation.ts`, `planning-navigation.ts`), die
 * Sichtbarkeitsregeln (Rechte, Schulschalter, Anwesenheitsmodus) bleiben in
 * der Seitenleiste und im Mehr-Menü: eine Seite, die die Person nicht sehen
 * darf, fällt aus ihrer Gruppe, und eine leere Gruppe fällt ganz weg.
 *
 * `staff-navigation.test.ts` schlägt an, wenn eine Katalogseite nirgends
 * oder doppelt steht.
 */

/**
 * Die dynamischen Bereiche mit eigenen Unterpunkten: Gruppen und Aufsichten
 * kommen aus der Sitzung, Datenverwaltung und Anmeldungen haben eine
 * Hub-Seite mit festen Unterseiten. Sie bleiben Akkordeons innerhalb ihrer
 * Gruppe.
 */
export type StaffNavSectionKey =
  "groups" | "supervisions" | "database" | "enrollments";

export type StaffNavGroupKey =
  "tagesbetrieb" | "eltern" | "team" | "planung" | "verwaltung";

export type StaffNavEntry =
  | { readonly kind: "page"; readonly href: string }
  | { readonly kind: "section"; readonly section: StaffNavSectionKey };

export interface StaffNavGroup {
  readonly key: StaffNavGroupKey;
  readonly label: string;
  /**
   * Symbol der Gruppenzeile; im eingeklappten Streifen ist es das einzige,
   * was von der Gruppe zu sehen ist. Bewusst keines der Seiten-Symbole
   * darunter, sonst stünden zwei gleiche Bilder untereinander.
   */
  readonly icon: PhosphorIcon;
  readonly entries: readonly StaffNavEntry[];
}

function page(href: string): StaffNavEntry {
  return { kind: "page", href };
}

function section(key: StaffNavSectionKey): StaffNavEntry {
  return { kind: "section", section: key };
}

/** Über den Gruppen: die Startseite der jeweiligen Rolle. */
export const STAFF_NAV_TOP: readonly StaffNavEntry[] = [
  page(STAFF_FLAT_PAGES.dashboard.href),
  page(STAFF_FLAT_PAGES.tagesplan.href),
];

export const STAFF_NAV_GROUPS: readonly StaffNavGroup[] = [
  {
    key: "tagesbetrieb",
    label: "Tagesbetrieb",
    icon: SunIcon,
    entries: [
      section("groups"),
      section("supervisions"),
      page(STAFF_FLAT_PAGES.studentSearch.href),
      page(STAFF_FLAT_PAGES.rooms.href),
      page(STAFF_FLAT_PAGES.activities.href),
      page(STAFF_FLAT_PAGES.substitutions.href),
      // Anfragen bündelt Eltern-Wünsche UND Anträge von Mitarbeitenden und
      // trägt den einzigen Zähler auf oberster Ebene: deshalb im immer
      // offenen Tagesbetrieb, nicht bei Eltern.
      page(STAFF_FLAT_PAGES.anfragen.href),
    ],
  },
  {
    key: "eltern",
    label: "Eltern",
    icon: UsersIcon,
    entries: [
      ...PARENT_SUB_PAGES.map((entry) => page(entry.href)),
      section("enrollments"),
    ],
  },
  {
    key: "team",
    label: "Team",
    icon: UsersFourIcon,
    entries: [
      page(STAFF_FLAT_PAGES.timeTracking.href),
      page(STAFF_FLAT_PAGES.calendar.href),
      page(STAFF_FLAT_PAGES.staff.href),
      ...COMMUNICATION_SUB_PAGES.map((entry) => page(entry.href)),
    ],
  },
  {
    key: "planung",
    label: "Planung",
    icon: CalendarBlankIcon,
    entries: PLANNING_SUB_PAGES.map((entry) => page(entry.href)),
  },
  {
    key: "verwaltung",
    label: "Verwaltung",
    icon: BuildingOfficeIcon,
    entries: [
      section("database"),
      page(STAFF_FLAT_PAGES.dayLog.href),
      page(STAFF_FLAT_PAGES.statistics.href),
      page(STAFF_FLAT_PAGES.dateien.href),
      page(STAFF_FLAT_PAGES.infoDisplays.href),
    ],
  },
];

/** Unter den Gruppen, angeheftet: Notfall, Hilfe, Einstellungen. */
export const STAFF_NAV_BOTTOM: readonly StaffNavEntry[] = [
  page(STAFF_FLAT_PAGES.emergency.href),
  page(STAFF_FLAT_PAGES.help.href),
  page(STAFF_FLAT_PAGES.settings.href),
];

/**
 * Beim ersten Besuch steht nur der Tagesbetrieb offen. Was die Person danach
 * auf- oder zuklappt, merkt sich der Browser (use-sidebar-groups.ts).
 */
export const STAFF_NAV_DEFAULT_OPEN_GROUPS: readonly StaffNavGroupKey[] = [
  "tagesbetrieb",
];

/**
 * Das Symbol jeder Seite, geteilt von Seitenleiste und Mehr-Menü. Die
 * flachen Seiten tragen ihr Symbol bereits an ihrem NAV_ITEM in der
 * Seitenleiste; die Katalogseiten (Eltern, Team, Planung) hatten mobil und
 * auf dem Desktop bisher verschiedene Bilder.
 */
export const STAFF_NAV_CONCEPTS: Readonly<Record<string, MotoConceptKey>> = {
  "/messages": "parentConversations",
  "/parent-announcements": "parentMessages",
  "/admin/guardian-approvals": "accounts",
  "/eltern/bankverbindungen": "payroll",
  "/meal-plan": "mealPlan",
  "/team-chat": "messages",
  "/tagesinformationen": "announcements",
  "/betreuungsplan": "carePlan",
  "/dienstplan": "staffPlan",
  "/vertretung": "substitution",
  "/lists": "lists",
  "/calendar-periods": "calendarPeriods",
  "/payroll": "payroll",
};

/**
 * Pfade, die ein Akkordeon-Bereich umfasst. Datenverwaltung deckt alle
 * Unterseiten mit ab (/database/students/import …), die Anmeldungen ihre
 * vier Katalogseiten.
 */
const SECTION_PATH_PREFIXES: Readonly<
  Record<StaffNavSectionKey, readonly string[]>
> = {
  groups: ["/ogs-groups"],
  supervisions: ["/active-supervisions"],
  database: ["/database"],
  enrollments: ENROLLMENT_SUB_PAGES.map((entry) => entry.href),
};

function entryMatchesPathname(entry: StaffNavEntry, pathname: string) {
  if (entry.kind === "section") {
    return SECTION_PATH_PREFIXES[entry.section].some((prefix) =>
      matchesPathPrefix(pathname, prefix),
    );
  }
  return matchesPathPrefix(pathname, entry.href);
}

/** Die Gruppe, in der eine Seite steht — `null` für Start- und Fußzeilen. */
export function getStaffNavGroupForHref(href: string): StaffNavGroupKey | null {
  for (const group of STAFF_NAV_GROUPS) {
    if (
      group.entries.some(
        (entry) => entry.kind === "page" && entry.href === href,
      )
    ) {
      return group.key;
    }
  }
  return null;
}

function isStudentDetailPath(pathname: string): boolean {
  return pathname.startsWith("/students/") && pathname !== "/students/search";
}

/**
 * Die Gruppe, zu der der aktuelle Pfad gehört, damit die Seitenleiste sie
 * aufklappen kann. Kinder-Detailseiten gehören zu der Gruppe, aus der man
 * gekommen ist (`?from=`), sonst zum Tagesbetrieb — dort steht „Alle
 * Kinder", der Standard-Rückweg der Breadcrumb. Planungsseiten zählen mit
 * ihren Alt-Pfaden (/timetables, /staff/dienstplan, /vertretungsplan).
 */
export function getStaffNavGroupForPathname(
  pathname: string,
  fromParam?: string | null,
): StaffNavGroupKey | null {
  if (isStudentDetailPath(pathname)) {
    return fromParam
      ? (getStaffNavGroupForPathname(fromParam) ?? "tagesbetrieb")
      : "tagesbetrieb";
  }
  const planningPage = getActivePlanningSubPage(pathname);
  if (planningPage) return getStaffNavGroupForHref(planningPage.href);
  for (const group of STAFF_NAV_GROUPS) {
    if (group.entries.some((entry) => entryMatchesPathname(entry, pathname))) {
      return group.key;
    }
  }
  return null;
}

/** Alle Seiten-Einträge des Baums, für Tests und das Mehr-Menü. */
export function listStaffNavPageHrefs(): readonly string[] {
  return [...STAFF_NAV_TOP, ...STAFF_NAV_GROUPS.flatMap((g) => g.entries)]
    .concat(STAFF_NAV_BOTTOM)
    .flatMap((entry) => (entry.kind === "page" ? [entry.href] : []));
}
