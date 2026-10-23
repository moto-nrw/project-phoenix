/**
 * Client for the compact per-phase registration export. Mirrors the
 * student-export download flow: POST to the Next.js proxy, then stream
 * the returned blob to a download. PDF = one block per submission
 * (print fallback); DOCX/XLSX = one flat row per child (full data).
 */

import { unavailableApiError } from "~/lib/api-error";
import { readEnrollmentError } from "~/lib/enrollment-api-error";
import { createLogger } from "~/lib/logger";
import { downloadBlob, filenameFromDisposition } from "~/lib/file-download";

const logger = createLogger({ component: "EnrollmentExportAPI" });

export type EnrollmentExportFormat = "pdf" | "docx" | "xlsx";

/**
 * Export a phase's registrations. When `childStatus` is given, the export
 * is limited to children with that exact status — mirroring the admin
 * list's status dropdown. Omit it (or pass undefined) to export all.
 */
export async function exportPhaseRegistrations(
  phaseId: string,
  format: EnrollmentExportFormat,
  childStatus?: string,
): Promise<void> {
  const body: { format: EnrollmentExportFormat; child_status?: string } = {
    format,
  };
  if (childStatus) body.child_status = childStatus;
  let response: Response;
  try {
    response = await fetch(
      `/api/enrollment/phases/${encodeURIComponent(phaseId)}/export`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      },
    );
  } catch (error) {
    throw unavailableApiError(error);
  }

  if (!response.ok) {
    throw await readEnrollmentError(
      response,
      "Export konnte nicht erstellt werden",
      logger,
      "enrollment_export_failed",
    );
  }

  downloadBlob(
    await response.blob(),
    filenameFromDisposition(response) ?? `anmeldungen.${format}`,
  );
}
