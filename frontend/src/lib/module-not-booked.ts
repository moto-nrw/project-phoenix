import { ApiError } from "~/lib/api-error";
import { isModuleKey, type ModuleKey } from "~/lib/modules";

/**
 * The module a request was refused for because the school did not book it
 * (`module.not_booked`, #4893), or `null` for every other failure. The
 * backend names the module key in `details.module`.
 */
export function notBookedModuleOf(error: unknown): ModuleKey | null {
  if (!(error instanceof ApiError) || error.code !== "module.not_booked") {
    return null;
  }
  const key = error.details?.module;
  return isModuleKey(key) ? key : null;
}

/**
 * The backend refused a module the navigation still showed as booked: the
 * moto team changed the booking since the school data was loaded. Reload it,
 * so navigation and the page explanation follow without a reload (#4894).
 */
export function announceModuleBookingStale(): void {
  if (typeof window === "undefined") return;
  window.dispatchEvent(new CustomEvent("phoenix:tenant-settings-stale"));
}
