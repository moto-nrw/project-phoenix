import { apiErrorFromResponse, transportFetch } from "~/lib/api-error";
import {
  downloadBlob,
  filenameFromDisposition,
  openBlobForPrint,
} from "~/lib/file-download";

export type EmergencySnapshotExportMode = "download" | "print";

export async function exportEmergencySnapshot(
  mode: EmergencySnapshotExportMode,
): Promise<void> {
  const response = await transportFetch("/api/emergency/snapshot/export", {
    method: "POST",
  });

  if (!response.ok) {
    throw await apiErrorFromResponse(
      response,
      "Emergency snapshot export failed",
    );
  }

  const blob = await response.blob();
  if (mode === "print") {
    openBlobForPrint(blob);
    return;
  }

  downloadBlob(blob, filenameFromDisposition(response) ?? "notfallliste.pdf");
}
