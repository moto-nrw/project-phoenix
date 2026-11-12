import { proxyGet } from "~/lib/parent/route-wrapper.server";
import type { BackendHolidayRegistrationEntries } from "~/lib/parent-holiday-registration-entries";

/**
 * Proxy GET /api/parent/me/holiday-registration-entries → backend
 * /parent/me/holiday-registration-entries (#6673): the holiday registrations
 * linked to the account. The account comes from the parent session, never
 * from the request.
 */
export const GET = proxyGet<BackendHolidayRegistrationEntries>(
  "/parent/me/holiday-registration-entries",
);
