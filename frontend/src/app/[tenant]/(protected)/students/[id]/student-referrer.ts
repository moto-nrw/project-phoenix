import { resolveDetailReferrer } from "~/lib/tenant-path";

/** Die Sammlungen, aus denen man die Kindakte öffnen kann. */
const STUDENT_REFERRER_PREFIXES = [
  "/students/search",
  "/database/students",
  "/rooms",
  "/active-supervisions",
  "/day-log",
  "/messages",
  "/absences",
  "/ogs-groups",
] as const;

/**
 * Der Rückweg aus der Kindakte und ihren Unterseiten. `from` kommt aus der
 * Adresszeile; nur ein interner Pfad einer dieser Sammlungen wird zum Link.
 */
export function resolveStudentReferrer(candidate: string | null): string {
  return resolveDetailReferrer(
    candidate,
    "/students/search",
    STUDENT_REFERRER_PREFIXES,
  );
}

/**
 * Der Rückweg heißt wie die Sammlung, aus der man kam: die Kinderdaten der
 * Datenverwaltung verlinken seit #3115 hierher, statt ein eigenes Pane zu
 * tragen.
 */
export function studentReferrerBackLabel(referrer: string): string {
  return referrer.startsWith("/database/students")
    ? "Zurück zu den Kinderdaten"
    : "Zurück zur Kinderübersicht";
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
