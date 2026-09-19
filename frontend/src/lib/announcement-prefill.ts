// Übergabe einer Kinderauswahl an den Verfasser der Elternmitteilungen.
//
// Eine andere Seite (der Rücklauf einer Anmeldephase, #3379) legt die Kinder
// ab und führt mit `?neu=kinder&vorbelegung=<token>` zur Mitteilungsliste.
// Der Token verweist nur im aktuellen Browser-Tab auf die Auswahl und bindet
// sie an den Mandanten. Dort wird sie genau einmal gelesen und sofort gelöscht.
// Namen gehören nicht in eine URL, und für achtzig Kinder wäre sie auch zu lang.
//
// sessionStorage kann leer zurückkommen oder werfen (privates Fenster,
// gesperrte Websitedaten). Dann öffnet sich der Verfasser ohne Vorbelegung;
// nichts davon ist ein Fehler.

export const ANNOUNCEMENT_PREFILL_PARAM = "neu";
export const ANNOUNCEMENT_PREFILL_STUDENTS = "kinder";
export const ANNOUNCEMENT_PREFILL_TOKEN_PARAM = "vorbelegung";

const STORAGE_PREFIX = "moto:announcement-prefill:";

export interface AnnouncementPrefillStudent {
  id: string;
  name: string;
}

export function stashAnnouncementStudents(
  tenantSlug: string,
  students: readonly AnnouncementPrefillStudent[],
): string | null {
  const token = globalThis.crypto?.randomUUID?.();
  if (!token) return null;
  try {
    window.sessionStorage.setItem(
      `${STORAGE_PREFIX}${token}`,
      JSON.stringify({ tenantSlug, students }),
    );
    return token;
  } catch {
    return null;
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

export function takeAnnouncementStudents(
  tenantSlug: string,
  token: string,
): AnnouncementPrefillStudent[] {
  try {
    const key = `${STORAGE_PREFIX}${token}`;
    const raw = window.sessionStorage.getItem(key);
    window.sessionStorage.removeItem(key);
    if (!raw) return [];
    const parsed: unknown = JSON.parse(raw);
    if (typeof parsed !== "object" || parsed === null) return [];
    const payload = parsed as Record<string, unknown>;
    if (payload.tenantSlug !== tenantSlug || !Array.isArray(payload.students)) {
      return [];
    }
    return payload.students.filter(isPrefillStudent);
  } catch {
    return [];
  }
}
