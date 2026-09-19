// Übergabe einer Kinderauswahl an den Verfasser der Elternmitteilungen.
//
// Eine andere Seite (der Rücklauf einer Anmeldephase, #3379) legt die Kinder
// ab und führt mit `?neu=kinder` zur Mitteilungsliste. Dort wird die Auswahl
// genau einmal gelesen und sofort gelöscht. Namen gehören nicht in eine URL,
// und für achtzig Kinder wäre sie auch zu lang.
//
// sessionStorage kann leer zurückkommen oder werfen (privates Fenster,
// gesperrte Websitedaten). Dann öffnet sich der Verfasser ohne Vorbelegung;
// nichts davon ist ein Fehler.

export const ANNOUNCEMENT_PREFILL_PARAM = "neu";
export const ANNOUNCEMENT_PREFILL_STUDENTS = "kinder";

const STORAGE_KEY = "moto:announcement-prefill-students";

export interface AnnouncementPrefillStudent {
  id: string;
  name: string;
}

export function stashAnnouncementStudents(
  students: readonly AnnouncementPrefillStudent[],
): boolean {
  try {
    window.sessionStorage.setItem(STORAGE_KEY, JSON.stringify(students));
    return true;
  } catch {
    return false;
  }
}

function isPrefillStudent(value: unknown): value is AnnouncementPrefillStudent {
  if (typeof value !== "object" || value === null) return false;
  const candidate = value as Record<string, unknown>;
  return (
    typeof candidate.id === "string" &&
    candidate.id !== "" &&
    typeof candidate.name === "string"
  );
}

export function takeAnnouncementStudents(): AnnouncementPrefillStudent[] {
  try {
    const raw = window.sessionStorage.getItem(STORAGE_KEY);
    window.sessionStorage.removeItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed: unknown = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed.filter(isPrefillStudent) : [];
  } catch {
    return [];
  }
}
