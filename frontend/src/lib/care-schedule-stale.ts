/**
 * Window event after a remote arrival or pickup change. The care schedule
 * editor keeps its data in local state, so that it can defer a refresh while
 * someone is editing; SWR invalidation does not reach it.
 */
export const CARE_SCHEDULE_STALE_EVENT = "phoenix:care-schedule-stale";

export function announceCareScheduleStale(): void {
  window.dispatchEvent(new CustomEvent(CARE_SCHEDULE_STALE_EVENT));
}
