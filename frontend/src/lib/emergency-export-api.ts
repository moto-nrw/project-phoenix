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
  // The print tab opens before the fetch, while the click still counts as a
  // user gesture: popup blockers refuse a window opened after an await.
  const printTarget = mode === "print" ? globalThis.open("", "_blank") : null;
  if (mode === "print" && !printTarget) {
    throw new Error("Der Druckdialog konnte nicht geöffnet werden.");
  }
  if (printTarget) printTarget.opener = null;

  try {
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
    if (printTarget) {
      openBlobForPrint(blob, printTarget);
      return;
    }

    downloadBlob(blob, filenameFromDisposition(response) ?? "notfallliste.pdf");
  } catch (error) {
    printTarget?.close();
    throw error;
  }
}
