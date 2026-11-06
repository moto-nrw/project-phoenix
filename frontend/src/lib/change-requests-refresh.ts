/**
 * Window event after a change request was decided, corrected or created
 * here, so pending counts and request lists elsewhere on the page reload.
 */
export const CHANGE_REQUESTS_REFRESH_EVENT = "change-requests-refresh";

export function announceChangeRequestsChanged(): void {
  window.dispatchEvent(new Event(CHANGE_REQUESTS_REFRESH_EVENT));
}
