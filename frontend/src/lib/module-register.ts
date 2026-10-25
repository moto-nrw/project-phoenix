/**
 * Modul-Register der Mitarbeiter-Oberfläche (#4894).
 *
 * Jede Seite der Navigation, jeder Bereich der Seitenleiste, jeder Baustein
 * der Startseite und jeder Reiter der Personalakte gehört genau einem
 * Zusatzmodul oder der Basis. Seitenleiste, mobile Navigation, Startseite,
 * Datenverwaltung und die Sperre direkt aufgerufener Seiten lesen diese eine
 * Liste; vorher stand jede Modulregel (NFC, Dienstplan am Betreuungsplan)
 * als eigener Sonderfall in Seitenleiste und Mehr-Menü.
 *
 * Die Zuordnung folgt der Routen-Zuordnung des Backends
 * (`backend/api/module_routes.go`, #4893) und dem Modulkatalog
 * (`docs/reference/modulschalter.md`). Zwei Abweichungen sind gewollt:
 *
 * - Aktivitäten (`/activities`, `/database/activities`) gehören hier zu
 *   NFC-Anwesenheit, im Backend zur Basis. Die Seiten waren schon vor den
 *   Modulen nur mit NFC zu sehen; eine Schule ohne NFC behält damit genau
 *   die Navigation, die sie kannte.
 * - Geräte (`/database/devices`) sperrt das Backend erst mit #4896. Die
 *   Seite war ebenso nur mit NFC zu sehen und bleibt es.
 *
 * Schul-Einstellungen wie `operations.meal_plan_enabled` oder
 * `timetable.enabled` bleiben Feinsteuerung in den Verbrauchern: eine Seite
 * erscheint nur, wenn ihr Modul gebucht UND die Einstellung an ist.
 *
 * Darstellung eines nicht gebuchten Moduls (Entscheidung in #4812):
 * Wer die Schule leitet, sieht den Eintrag mit Schloss; der Eintrag führt auf
 * die Seite, und die erklärt, wer das Modul buchen kann. Alle anderen sehen
 * nichts. `module-register.test.ts` schlägt an, wenn eine Seite der
 * Navigation oder ein Baustein hier fehlt.
 */

import type { Session } from "next-auth";

import { leadsSchool } from "~/lib/auth-utils";
import type { ModuleKey } from "~/lib/modules";
import { matchesPathPrefix } from "~/lib/section-navigation";
import type { StaffNavSectionKey } from "~/lib/staff-navigation";

/** moto Basis: hat jede Schule, hat keinen Schalter. */
export const BASIS = "basis";

export type ModuleScope = ModuleKey | typeof BASIS;

/**
 * Jede Seite der Mitarbeiter-Navigation mit ihrem Modul. Alte Adressen, die
 * noch auf eine Seite führen, stehen mit darin, damit die Sperre einer
 * direkt aufgerufenen Seite auch sie trifft.
 */
export const PAGE_MODULES: Readonly<Record<string, ModuleScope>> = {
  "/home": BASIS,

  // Tagesbetrieb
  "/tagesplan": BASIS,
  "/ogs-groups": BASIS,
  "/students/search": BASIS,
  "/active-supervisions": BASIS,
  "/rooms": BASIS,
  "/activities": "nfc_anwesenheit",
  "/substitutions": BASIS,
  // Der Anfragen-Eingang mischt Eltern-App und Online-Anmeldung; offene
  // Anfragen schließt die Schule auch ohne beide ab (wie im Backend).
  "/anfragen": BASIS,

  // Eltern
  "/messages": "eltern_app",
  "/parent-announcements": "eltern_app",
  "/admin/guardian-approvals": "eltern_app",
  // Bankdaten der Eltern gibt es heute ohne Abrechnung (wie im Backend).
  "/eltern/bankverbindungen": BASIS,
  "/meal-plan": "mittagessen",

  // Anmeldungen, auch die Betreuungsangebote: ihre Verwaltung liegt im
  // Bereich Anmeldungen.
  "/admin/enrollments": "online_anmeldung",
  "/enrollment-phases": "online_anmeldung",
  "/care-offerings": "online_anmeldung",
  "/enrollment-form": "online_anmeldung",

  // Team
  "/time-tracking": "zeiterfassung",
  "/mein-dienstplan": "dienstplan",
  "/calendar": BASIS,
  "/staff": BASIS,
  "/team-chat": BASIS,
  "/tagesinformationen": BASIS,
  // Umfragen an das Team und an Eltern (#4781): Basis wie im Backend; eine
  // Umfrage an Eltern erreicht sie nur über die Eltern-App.
  "/umfragen": BASIS,

  // Planung. Der Dienstplan setzt nur Basis voraus und hängt nicht mehr am
  // Betreuungsplan (`timetable.enabled`, Entscheidung in #4812).
  "/betreuungsplan": BASIS,
  "/timetables": BASIS,
  "/dienstplan": "dienstplan",
  "/staff/dienstplan": "dienstplan",
  "/vertretung": BASIS,
  "/vertretungsplan": BASIS,
  "/lists": BASIS,
  "/calendar-periods": BASIS,
  // Lohnabrechnung aus der Zeiterfassung (Lohnbüro-Versand, Personalnummer).
  "/payroll": "zeiterfassung",

  // Verwaltung
  "/database": BASIS,
  "/database/students": BASIS,
  "/database/personal": BASIS,
  // Der Import ändert Stundenkonto und Urlaubssaldo und gehört deshalb zur
  // Zeiterfassung, nicht zu den Basis-Stammdaten des Personals.
  "/database/personal/opening-balances": "zeiterfassung",
  "/database/rooms": BASIS,
  "/database/activities": "nfc_anwesenheit",
  "/database/categories": BASIS,
  "/database/planning-tracks": BASIS,
  "/database/shift-types": "dienstplan",
  // Abwesenheitsarten des Teams: Urlaub und Krankheit gehören zur
  // Zeiterfassung.
  "/database/absence-types": "zeiterfassung",
  "/database/groups": BASIS,
  "/database/roles": BASIS,
  "/database/devices": "nfc_anwesenheit",
  "/database/permissions": BASIS,
  "/database/grade-transitions": BASIS,
  "/database/exports": BASIS,
  "/day-log": BASIS,
  "/statistics": BASIS,
  "/dateien": BASIS,
  "/info-displays": BASIS,

  // Unten angeheftet
  "/emergency": BASIS,
  "/help": BASIS,
  "/settings": BASIS,
};

/** Die aufklappbaren Bereiche der Seitenleiste. */
export const SECTION_MODULES: Readonly<
  Record<StaffNavSectionKey, ModuleScope>
> = {
  groups: BASIS,
  supervisions: BASIS,
  database: BASIS,
  enrollments: "online_anmeldung",
};

/**
 * Jeder Baustein der Startseite (`HomeBlockKey` in home-blocks.ts; der Test
 * hält beide Listen gleich, ein Typ-Import wäre ein Kreis).
 */
export const HOME_BLOCK_MODULES: Readonly<Record<string, ModuleScope>> = {
  "tile.students_present": BASIS,
  "tile.students_in_rooms": BASIS,
  "tile.students_in_transit": BASIS,
  "tile.students_on_playground": BASIS,
  "tile.students_sick": BASIS,
  "tile.students_excused": BASIS,
  "tile.students_at_school": BASIS,
  "tile.students_home": BASIS,
  "tile.active_activities": "nfc_anwesenheit",
  "tile.capacity_utilization": BASIS,
  "section.birthdays": BASIS,
  "section.recent_activity": BASIS,
  "section.current_activities": "nfc_anwesenheit",
  "section.active_groups": BASIS,
  "section.my_day": BASIS,
  "section.my_group": BASIS,
  "section.students": BASIS,
  "section.staff_notices": BASIS,
  "section.day_flow": BASIS,
  "section.open_requests": BASIS,
  "section.reminders": BASIS,
  "section.staff_today": BASIS,
  // Zählt Eltern-Nachrichten und den Team-Chat. Der Team-Chat ist Basis;
  // die Eltern-Nachrichten zählen nur, wenn `/messages` offen ist.
  "section.messages": BASIS,
};

/**
 * Reiter der Personalakte (`/staff/[id]`), die zu einem Modul gehören. Ein
 * Reiter ohne Eintrag ist Basis. Das Arbeitszeitmodell gehört zur
 * Zeiterfassung (Entscheidung in #4812).
 */
export const STAFF_TAB_MODULES: Readonly<Record<string, ModuleKey>> = {
  // Die Übersicht zeigt Stundenkonto und Woche aus der Zeiterfassung.
  uebersicht: "zeiterfassung",
  zeiterfassung: "zeiterfassung",
  arbeitszeitmodell: "zeiterfassung",
  abwesenheiten: "zeiterfassung",
  stammdaten: "personalakte",
  dokumente: "personalakte",
};

/**
 * Das Modul hinter einer Adresse der Mitarbeiter-Oberfläche, oder `null` für
 * Basis und unbekannte Adressen. Der längste passende Pfad gewinnt, damit
 * `/staff/dienstplan` nicht an `/staff` hängen bleibt.
 */
export function moduleOfPath(pathname: string): ModuleKey | null {
  let best: string | null = null;
  for (const href of Object.keys(PAGE_MODULES)) {
    if (!matchesPathPrefix(pathname, href)) continue;
    if (best === null || href.length > best.length) best = href;
  }
  const scope = best === null ? BASIS : PAGE_MODULES[best];
  return scope === undefined || scope === BASIS ? null : scope;
}

/**
 * Wie ein Eintrag eines Moduls erscheint: `open` (gebucht oder Basis),
 * `locked` (nicht gebucht, die Person leitet die Schule und sieht das
 * Schloss) oder `hidden` (nicht gebucht, alle anderen).
 */
export type ModuleVisibility = "open" | "locked" | "hidden";

export function moduleVisibility(
  scope: ModuleScope | undefined,
  booked: readonly ModuleKey[],
  seesLocked: boolean,
): ModuleVisibility {
  if (scope === undefined || scope === BASIS || booked.includes(scope)) {
    return "open";
  }
  return seesLocked ? "locked" : "hidden";
}

/** Gebucht oder Basis: der Eintrag ist ohne Schloss erreichbar. */
export function isScopeBooked(
  scope: ModuleScope | undefined,
  booked: readonly ModuleKey[],
): boolean {
  return moduleVisibility(scope, booked, false) === "open";
}

/**
 * Wer ein nicht gebuchtes Modul mit Schloss sieht: wer die Schule leitet
 * (Adminzuschnitt oder `config:manage`). Diese Personen sehen auch die
 * gebuchten Module unter Einstellungen und wenden sich ans moto-Team.
 */
export function seesLockedModules(session: Session | null): boolean {
  return leadsSchool(session);
}
