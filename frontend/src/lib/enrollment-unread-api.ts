/**
 * Ungelesene Anmeldungen (#3778): der Zähler und sein Neu-zählen-Ereignis.
 * Eigenes kleines Modul, weil Seitenleiste und mobiles Menü auf jeder Seite
 * geladen sind und nicht die ganze Anmeldungs-API mitziehen sollen.
 */

/** Fenster-Ereignis, nach dem das Anmeldungen-Badge neu zählt. */
export const ENROLLMENTS_UNREAD_REFRESH_EVENT = "enrollments-unread-refresh";

/** Lässt das Anmeldungen-Badge neu zählen. */
export function announceEnrollmentReadChange() {
  globalThis.window?.dispatchEvent(new Event(ENROLLMENTS_UNREAD_REFRESH_EVENT));
}

/** Ungelesene Anmeldungen der Person; 0 bei Fehlern oder ohne Recht. */
export async function fetchUnreadEnrollmentCount(): Promise<number> {
  try {
    const response = await fetch(
      "/api/enrollment/admin/requests/unread-count",
      { cache: "no-store" },
    );
    if (!response.ok) return 0;
    const envelope = (await response.json()) as {
      data?: { unread_count?: number };
    };
    return envelope.data?.unread_count ?? 0;
  } catch {
    return 0;
  }
}
