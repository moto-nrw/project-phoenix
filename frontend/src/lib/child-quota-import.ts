import type { AlertType } from "~/components/ui/alert";

/**
 * Kinderkontingent in der Import-Vorschau (#3571). Fehlt bei einer Schule
 * ohne Kinderkontingent. Die Namen der Plätze folgen den Details der
 * 409-Ablehnung `students.child_quota_reached`.
 */
export interface ImportChildQuota {
  booked_places: number;
  occupied_places: number;
  requested_places: number;
  free_places: number;
  fits: boolean;
}

function childCount(count: number): string {
  return count === 1 ? "1 Kind" : `${count} Kinder`;
}

function freeSentence(free: number, fits: boolean): string {
  if (free === 0) return "Das Kinderkontingent ist voll.";
  const verb = free === 1 ? "ist" : "sind";
  return `Im Kinderkontingent ${verb} ${fits ? "noch" : "nur noch"} ${free} frei.`;
}

/**
 * Hinweis der Vorschau: wie viele Kinder der Import neu anlegt und wie viel
 * das Kinderkontingent noch aufnimmt. Passt es nicht, startet der Import
 * nicht. Ein Import, der nur Kinder ändert, braucht keinen Hinweis.
 */
export function importChildQuotaNotice(
  quota: ImportChildQuota | null | undefined,
): { type: Extract<AlertType, "info" | "error">; message: string } | null {
  if (!quota || quota.requested_places === 0) return null;
  const added = `Der Import würde ${childCount(quota.requested_places)} hinzufügen.`;
  const free = freeSentence(quota.free_places, quota.fits);
  if (quota.fits) return { type: "info", message: `${added} ${free}` };
  return {
    type: "error",
    message: `${added} ${free} Der Import startet darum nicht. Nehmen Sie Kinder aus der Datei heraus oder melden Sie sich beim moto-Team.`,
  };
}
