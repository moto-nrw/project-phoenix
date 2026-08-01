/**
 * Shared plumbing for the export routes that hand back a rendered file.
 *
 * Every export client needs the same three things — read the filename the
 * backend chose, save the blob, or open it in a print dialog — and each one
 * had grown its own copy. They are here once so a fix (the popup-blocker
 * dance below is the kind that gets fixed in one place and forgotten in the
 * others) lands everywhere.
 */

/** The filename the backend put in Content-Disposition, if any. */
export function filenameFromDisposition(response: Response): string | null {
  const disposition = response.headers.get("content-disposition");
  const match = /filename="([^"]+)"/.exec(disposition ?? "");
  return match?.[1] ?? null;
}

/** Saves a blob under the given name and releases the object URL. */
export function downloadBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.append(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

/**
 * Opens a blob in a tab and triggers the print dialog.
 *
 * `target` is a tab the caller already opened synchronously inside the
 * click handler. That matters: after an `await`, popup blockers (Safari,
 * strict settings) return null from `window.open`, so an export that fetches
 * first can only print into a tab opened before the fetch. Callers that do no
 * async work before printing may omit it and let this open the tab.
 */
export function openBlobForPrint(blob: Blob, target?: Window | null): void {
  const url = URL.createObjectURL(blob);
  const tab = target ?? globalThis.open(url, "_blank");
  if (!tab) {
    URL.revokeObjectURL(url);
    throw new Error("Der Druckdialog konnte nicht geöffnet werden.");
  }
  if (target) {
    target.location.href = url;
  }
  // PDF viewer documents fire no reliable load event across browsers, so
  // printing keeps the pragmatic delay every print export here has used.
  globalThis.setTimeout(() => {
    tab.focus();
    tab.print();
    URL.revokeObjectURL(url);
  }, 250);
}
