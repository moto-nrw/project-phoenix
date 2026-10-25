import { proxyGet } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface FamilyResponse {
  guardian: {
    first_name: string;
    last_name: string;
    email: string;
    phone: string;
  };
  children: {
    student_id: string;
    school_id: string;
    first_name: string;
    last_name: string;
    birth_date?: string;
  }[];
}

/**
 * Proxy GET /api/parent/holiday-registrations/{token}/family → backend
 * /parent/holiday-registrations/{token}/family (#4662). Forwards the parent
 * session; without one the route wrapper answers 401 and the holiday
 * registration starts empty.
 */
export const GET = proxyGet<FamilyResponse>(
  (params) =>
    `/parent/holiday-registrations/${requirePathSegmentParam(params, "token")}/family`,
);
