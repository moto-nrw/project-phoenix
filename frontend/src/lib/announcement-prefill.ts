// Übergabe einer Kinderauswahl an den Verfasser der Elternmitteilungen.
//
// Eine andere Seite (der Rücklauf einer Anmeldephase, #3379, oder die
// Kindersuche, #4779) legt die Kinder ab und führt mit
// `?neu=kinder&vorbelegung=<token>` zur Mitteilungsliste.
// Der Token verweist auf die Auswahl und bindet sie an den Mandanten. Dort
// wird sie genau einmal gelesen und sofort gelöscht. Namen gehören nicht in
// eine URL, und für achtzig Kinder wäre sie auch zu lang.
//
// Die Auswahl liegt nur im Arbeitsspeicher dieses Tabs (#5000), nie im
// Browser-Speicher: Namen von Kindern überleben so weder das Schließen des
// Tabs noch einen Logout (clearPersonalStorage räumt sie mit ab). Die
// Übergabe läuft über eine Navigation im selben Tab; nach dem Neuladen oder
// in einem anderen Tab öffnet sich der Verfasser ohne Vorbelegung, und
// nichts davon ist ein Fehler. Es liegt höchstens eine Auswahl bereit.

export const ANNOUNCEMENT_PREFILL_PARAM = "neu";
export const ANNOUNCEMENT_PREFILL_STUDENTS = "kinder";
export const ANNOUNCEMENT_PREFILL_TOKEN_PARAM = "vorbelegung";

export interface AnnouncementPrefillStudent {
  id: string;
  name: string;
}

interface PendingSelection {
  readonly token: string;
  readonly tenantSlug: string;
  readonly students: readonly AnnouncementPrefillStudent[];
}

let pending: PendingSelection | null = null;

export function stashAnnouncementStudents(
  tenantSlug: string,
  students: readonly AnnouncementPrefillStudent[],
): string | null {
  const token = globalThis.crypto?.randomUUID?.();
  if (!token) return null;
  pending = {
    token,
    tenantSlug,
    students: students.map(({ id, name }) => ({ id, name })),
  };
  return token;
}

/**
 * Pfad zum Verfasser mit der abgelegten Auswahl. Ohne Token (keine Ablage
 * möglich) öffnet sich nur die Liste; die Kinder lassen sich dort von Hand
 * wählen. `art` wählt den Reiter, zum Beispiel `elternbriefe`.
 */
export function announcementPrefillPath(
  token: string | null,
  art?: string,
): string {
  const query = new URLSearchParams();
  if (art) query.set("art", art);
  if (token) {
    query.set(ANNOUNCEMENT_PREFILL_PARAM, ANNOUNCEMENT_PREFILL_STUDENTS);
    query.set(ANNOUNCEMENT_PREFILL_TOKEN_PARAM, token);
  }
  const serialized = query.toString();
  return serialized
    ? `/parent-announcements?${serialized}`
    : "/parent-announcements";
}

export function takeAnnouncementStudents(
  tenantSlug: string,
  token: string,
): AnnouncementPrefillStudent[] {
  const selection = pending;
  if (selection?.token !== token) return [];
  pending = null;
  if (selection.tenantSlug !== tenantSlug) return [];
  return selection.students.map(({ id, name }) => ({ id, name }));
}

/** Verwirft eine noch nicht abgeholte Auswahl (Logout, Kontowechsel). */
export function clearAnnouncementStudents(): void {
  pending = null;
}
