// Pure helpers for showing a file: no API, no state. Kept apart from the
// Dateiablage client (`files-api.ts`), which creates its service on load, so
// the kit's AttachmentList can format a file without loading that client
// (#6582).

/** Browsers render these themselves; office files always download. */
export function isViewableInBrowser(contentType: string): boolean {
  return (
    contentType === "application/pdf" ||
    contentType === "image/png" ||
    contentType === "image/jpeg" ||
    contentType === "image/jpg"
  );
}

/** Human-readable size for the quota line and the file table. */
export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  if (bytes < 1024 * 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}
