import { trackEvent } from "~/lib/analytics";
import {
  downloadBlob,
  filenameFromDisposition,
  openBlobForPrint,
} from "~/lib/file-download";

export type EmergencySnapshotExportMode = "download" | "print";

export async function exportEmergencySnapshot(
  mode: EmergencySnapshotExportMode,
): Promise<void> {
  const response = await fetch("/api/emergency/snapshot/export", {
    method: "POST",
  });

  if (!response.ok) {
    throw new Error(await response.text());
  }

  trackEvent("data_exported", { export_type: "emergency", format: "pdf" });

  const blob = await response.blob();
  if (mode === "print") {
    openBlobForPrint(blob);
    return;
  }

  downloadBlob(blob, filenameFromDisposition(response) ?? "notfallliste.pdf");
}
