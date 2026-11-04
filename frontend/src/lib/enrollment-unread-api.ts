/**
 * Ungelesene Anmeldungen (#3778): der Zähler und sein Neu-zählen-Auslöser.
 * Eigenes kleines Modul, weil Seitenleiste und mobiles Menü auf jeder Seite
 * geladen sind und nicht die ganze Anmeldungs-API mitziehen sollen.
 */

import { mutate } from "~/lib/swr";

/**
 * SWR-Schlüssel des Zählers; useEnrollmentsUnread hängt das Konto an,
 * useSWRAuth stellt die Schule davor.
 */
export const ENROLLMENTS_UNREAD_SWR_KEY = "enrollments-unread-count";

function isEnrollmentsUnreadKey(key: unknown): boolean {
  return (
    typeof key === "string" && key.includes(`${ENROLLMENTS_UNREAD_SWR_KEY}:`)
  );
}

/**
 * Lässt das Anmeldungen-Badge neu zählen: ein Request je Konto, egal wie
 * viele Stellen den Zähler gerade zeigen (#4989). Ein Ereignis je Stelle
 * hätte jede einzeln laden lassen.
 */
export function announceEnrollmentReadChange() {
  void mutate(isEnrollmentsUnreadKey);
}

/**
 * Ungelesene Anmeldungen der Person; 0 bei Fehlern oder ohne Recht.
 * Bewusst still: das Badge lädt ohne Handlung der Person (kein Toast ohne
 * Handlung, #2501), 0 blendet es nur aus. Der Bereich selbst zeigt seine
 * Ladefehler vor Ort.
 */
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
