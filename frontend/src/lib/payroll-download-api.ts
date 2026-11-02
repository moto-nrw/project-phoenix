// Download der Monatsdatei über den Link aus der E-Mail an das Lohnbüro
// (#4805). Öffentlich: keine Sitzung, der Link ist die Berechtigung. Nur ein
// POST öffnet ihn, und das genau einmal.

import { apiErrorFromResponse, transportFetch } from "./api-error";
import { downloadBlob, filenameFromDisposition } from "./file-download";

const FALLBACK_FILENAME = "monatsdatei";

/**
 * Der Dateiname aus Content-Disposition. Go setzt einen schlichten Namen wie
 * `zeitkonten_2026-09.csv` ohne Anführungszeichen; den liest der gemeinsame
 * Helfer bewusst nicht.
 */
function payrollFilename(response: Response): string {
  const quoted = filenameFromDisposition(response);
  if (quoted) return quoted;
  const disposition = response.headers.get("content-disposition") ?? "";
  const bare = /(?:^|;)\s*filename=([^";\s]+)/.exec(disposition);
  return bare?.[1] ?? FALLBACK_FILENAME;
}

/**
 * Holt die Datei und speichert sie. Ein abgelaufener, verbrauchter oder
 * unbekannter Link wirft einen ApiError mit dem Code
 * `workforce.payroll_download_unavailable`.
 */
export async function downloadPayrollFile(link: string): Promise<void> {
  const response = await transportFetch(
    `/api/public/payroll-download/${encodeURIComponent(link)}`,
    { method: "POST", cache: "no-store" },
  );
  if (!response.ok) {
    throw await apiErrorFromResponse(
      response,
      `Payroll download failed: ${response.status}`,
    );
  }
  const blob = await response.blob();
  downloadBlob(blob, payrollFilename(response));
}
