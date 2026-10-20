import { resolveDetailReferrer } from "~/lib/tenant-path";

/**
 * Die Sammlungen, aus denen man die Kindakte öffnen kann, und wie der
 * Rückweg dorthin heißt. Der Link zeigt den Text sichtbar an, also muss er
 * das Ziel nennen, aus dem man kam.
 */
const STUDENT_REFERRERS = [
  ["/students/search", "Zurück zur Kinderübersicht"],
  ["/database/students", "Zurück zu den Kinderdaten"],
  ["/rooms", "Zurück zu den Räumen"],
  ["/active-supervisions", "Zurück zur Aktuellen Aufsicht"],
  ["/day-log", "Zurück zur Tagesauswertung"],
  ["/messages", "Zurück zu Nachrichten"],
  ["/absences", "Zurück zu den Abwesenheiten"],
  ["/ogs-groups", "Zurück zur Gruppe"],
] as const;

const DEFAULT_STUDENT_REFERRER = "/students/search";

/**
 * Der Rückweg aus der Kindakte und ihren Unterseiten. `from` kommt aus der
 * Adresszeile; nur ein interner Pfad einer dieser Sammlungen wird zum Link.
 */
export function resolveStudentReferrer(candidate: string | null): string {
  return resolveDetailReferrer(
    candidate,
    DEFAULT_STUDENT_REFERRER,
    STUDENT_REFERRERS.map(([prefix]) => prefix),
  );
}

/** Sichtbarer Text des Rückwegs zu einem aufgelösten `referrer`. */
export function studentReferrerBackLabel(referrer: string): string {
  const match = STUDENT_REFERRERS.find(
    ([prefix]) =>
      referrer === prefix ||
      referrer.startsWith(`${prefix}/`) ||
      referrer.startsWith(`${prefix}?`) ||
      referrer.startsWith(`${prefix}#`),
  );
  return match?.[1] ?? "Zurück zur Kinderübersicht";
}

/** Rückweg einer Unterseite der Kindakte in deren Reiter „Historie". */
export function studentHistoryBack(
  studentId: string,
  referrer: string,
  failed: boolean,
): { readonly backHref: string; readonly backLabel: string } {
  // Im Fehlerfall führt der Rückweg auf die Liste, sonst auf die Kindakte in
  // den Reiter, aus dem diese Unterseite geöffnet wurde (#1501). `from` trägt
  // weiter den Rückweg der Kindakte selbst.
  return failed
    ? { backHref: referrer, backLabel: studentReferrerBackLabel(referrer) }
    : {
        backHref: `/students/${studentId}?from=${encodeURIComponent(referrer)}&tab=historie`,
        backLabel: "Zurück zum Kinderprofil",
      };
}
