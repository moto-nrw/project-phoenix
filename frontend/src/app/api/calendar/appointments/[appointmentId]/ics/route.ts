import { type NextRequest } from "next/server";
import { auth, uncachedAuth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { streamIcsDownload } from "~/lib/ics-download.server";
import {
  createUnauthorizedResponse,
  extractParams,
  isStringParam,
  type RouteContext,
} from "~/lib/route-wrapper-utils.server";
import { problemResponse } from "~/lib/api-helpers.server";

export const runtime = "nodejs";

// Streams the appointment as a text/calendar (.ics) download. Kept out of the
// JSON route-wrapper because the body is an iCalendar document, not JSON, but it
// is wrapped in `withTenantAuth` (response-aware auth) so a session token
// refreshed during the download — either by the outer wrapper or by
// streamIcsDownload's retry-on-401 — is persisted back as a Set-Cookie on the
// response. Without it, the rotated cookie is dropped and a later download (or
// any request) after the refresh could fail with a stale token.
export const GET = withTenantAuth(
  async (request: NextRequest, context: RouteContext): Promise<Response> => {
    const session = await auth();
    if (!session?.user?.token) {
      return createUnauthorizedResponse(request);
    }
    const params = await extractParams(request, context);
    const appointmentId = params.appointmentId;
    if (!isStringParam(appointmentId) || appointmentId.length === 0) {
      return problemResponse(400, "Appointment ID is required", request);
    }
    return streamIcsDownload(
      `/api/calendar/appointments/${encodeURIComponent(appointmentId)}/ics`,
      session.user.token,
      uncachedAuth,
    );
  },
);
