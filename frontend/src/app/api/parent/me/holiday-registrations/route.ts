import { proxyGet } from "~/lib/parent/route-wrapper.server";

interface BackendHolidayOffer {
  token: string;
  name: string;
  traeger_name: string;
  registration_opens_on: string;
  registration_closes_on: string;
  first_day?: string;
  last_day?: string;
  open: boolean;
  children: { student_id: string; first_name: string }[];
}

/**
 * Proxy GET /api/parent/me/holiday-registrations → backend
 * /parent/me/holiday-registrations (#6671): the holiday periods of the
 * Träger of the account's children a parent can register for. The account
 * comes from the parent session, never from the request.
 */
export const GET = proxyGet<BackendHolidayOffer[]>(
  "/parent/me/holiday-registrations",
);
