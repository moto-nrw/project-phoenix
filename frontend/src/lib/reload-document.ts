/**
 * Loads the whole page again from the server: every component state, open
 * dialog and cached answer goes, and the page's first load runs anew. Its
 * own module so tests can replace it (jsdom cannot navigate).
 */
export function reloadDocument(): void {
  globalThis.location.reload();
}
