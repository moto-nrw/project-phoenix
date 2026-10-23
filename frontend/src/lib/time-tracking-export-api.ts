import { apiErrorFromResponse, transportFetch } from "~/lib/api-error";
import {
  downloadBlob,
  filenameFromDisposition,
  openBlobForPrint,
} from "~/lib/file-download";

export type TimeTrackingExportFormat = "pdf" | "xlsx" | "csv";

/**
 * Downloads the Zeitnachweis of one person for a period. The own time
 * tracking uses the same route with the signed-in person's staff ID; the
 * backend allows that without the manage permission.
 */
export async function exportStaffTimeTracking(
  staffId: string,
  from: string,
  to: string,
  format: TimeTrackingExportFormat,
  printTarget?: Window,
): Promise<void> {
  const params = new URLSearchParams({ from, to, format });
  const response = await transportFetch(
    `/api/staff/${encodeURIComponent(staffId)}/time-tracking/export?${params.toString()}`,
  );
  if (!response.ok) {
    throw await apiErrorFromResponse(response, "time tracking export failed");
  }
  const blob = await response.blob();
  if (printTarget) {
    openBlobForPrint(blob, printTarget);
    return;
  }
  downloadBlob(
    blob,
    filenameFromDisposition(response) ?? `zeitnachweis-${from}-${to}.${format}`,
  );
}
