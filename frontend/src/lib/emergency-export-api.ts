import { trackEvent } from "~/lib/analytics";

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
  const url = URL.createObjectURL(blob);
  if (mode === "print") {
    openForPrint(url);
    return;
  }

  const link = document.createElement("a");
  link.href = url;
  link.download = filenameFromDisposition(response) ?? "notfallliste.pdf";
  document.body.append(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

function openForPrint(url: string) {
  const target = globalThis.open(url, "_blank");
  if (!target) {
    URL.revokeObjectURL(url);
    throw new Error("Der Druckdialog konnte nicht geöffnet werden.");
  }
  globalThis.setTimeout(() => {
    target.focus();
    target.print();
    URL.revokeObjectURL(url);
  }, 250);
}

function filenameFromDisposition(response: Response): string | null {
  const disposition = response.headers.get("content-disposition");
  const match = /filename="([^"]+)"/.exec(disposition ?? "");
  return match?.[1] ?? null;
}
