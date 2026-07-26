import { trackEvent } from "~/lib/analytics";

export type RoomSnapshotExportFormat = "pdf" | "docx" | "xlsx";

export interface RoomSnapshotExportRequest {
  format: RoomSnapshotExportFormat;
  title: string;
  /**
   * Rooms to include. Omit the key entirely to export every room — the backend
   * models this as a pointer and distinguishes an absent key (no filter) from
   * an empty list (matches nothing, so the document comes out empty).
   */
  room_ids?: number[];
  include_transit: boolean;
}

export async function exportRoomSnapshot(
  request: RoomSnapshotExportRequest,
): Promise<void> {
  const response = await fetch("/api/rooms/export", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request),
  });

  if (!response.ok) {
    throw new Error(await response.text());
  }

  trackEvent("data_exported", {
    export_type: "rooms",
    format: request.format,
  });

  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download =
    filenameFromDisposition(response) ?? `wer-ist-wo.${request.format}`;
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
