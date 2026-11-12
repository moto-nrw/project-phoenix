/**
 * Refusals of data the screen already shows (#6801). A revalidation that the
 * server refuses (401/403) keeps SWR's data next to the error; nothing
 * rejects, so a caller that reloads through SWR cannot see the refusal.
 * The SWR middleware counts these refusals here, and
 * `useRefreshAfterWrite` compares the count around its reload.
 */

/** The server refused the account access (401/403), whatever the code. */
export function isAccessRefusal(error: unknown): error is { status: number } {
  const status =
    error && typeof error === "object"
      ? (error as { status?: unknown }).status
      : undefined;
  return status === 401 || status === 403;
}

let shownDataRefusals = 0;

/** A refused revalidation of a key that still holds shown data. */
export function noteShownDataRefusal(): void {
  shownDataRefusals++;
}

/** How many refusals of shown data this tab has seen so far. */
export function shownDataRefusalCount(): number {
  return shownDataRefusals;
}
