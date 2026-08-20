import type { StatusBadgeTone } from "~/components/ui/status-badge";
import type { AdminEnrollmentChangeRequestStatus } from "~/lib/enrollment-admin-api";

/**
 * Beschriftung und Farbe eines Anmeldungsänderungs-Status — an einer Stelle,
 * weil zwei Oberflächen denselben Status zeigen: die Zeile im Anfragen-Modul
 * (#2435) und die Detailansicht mit Rückfrage-Dialog. Zwei Kopien drifteten
 * schon einmal auseinander, und ein Status ohne Eintrag erscheint dann roh in
 * englischer Schreibweise.
 *
 * `cancelled` erzeugt heute niemand; die Spalte lässt den Wert aber zu, also
 * trägt er hier einen Namen statt durchzufallen.
 */
export const ENROLLMENT_CHANGE_REQUEST_STATUS_META: Record<
  AdminEnrollmentChangeRequestStatus,
  { readonly label: string; readonly tone: StatusBadgeTone }
> = {
  pending_review: { label: "Wartet auf Prüfung", tone: "blue" },
  needs_parent_response: { label: "Rückfrage offen", tone: "orange" },
  approved: { label: "Freigegeben", tone: "green" },
  rejected: { label: "Abgelehnt", tone: "red" },
  cancelled: { label: "Zurückgezogen", tone: "gray" },
};

/** Fällt auf den rohen Wert zurück, statt eine leere Markierung zu zeigen. */
export function enrollmentChangeRequestStatusMeta(status: string): {
  label: string;
  tone: StatusBadgeTone;
} {
  return (
    ENROLLMENT_CHANGE_REQUEST_STATUS_META[
      status as AdminEnrollmentChangeRequestStatus
    ] ?? { label: status, tone: "gray" }
  );
}
