/**
 * Client for the compact per-phase registration export. Mirrors the
 * student-export download flow: POST to the Next.js proxy, then stream
 * the returned blob to a download. PDF = one block per submission
 * (print fallback); DOCX/XLSX = one flat row per child (full data).
 */

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
  const response = await fetch(
    `/api/enrollment/phases/${encodeURIComponent(phaseId)}/export`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    },
  );

  if (!response.ok) {
    throw new Error(await response.text());
  }

  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filenameFromDisposition(response) ?? `anmeldungen.${format}`;
  document.body.append(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

function filenameFromDisposition(response: Response): string | null {
  const disposition = response.headers.get("content-disposition");
  const match = /filename="([^"]+)"/.exec(disposition ?? "");
  return match?.[1] ?? null;
}
