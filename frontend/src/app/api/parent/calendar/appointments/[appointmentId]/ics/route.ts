import { type NextRequest } from "next/server";
import { parentAuth, uncachedParentAuth } from "~/server/auth/parent";
import { withParentAuth } from "~/server/auth/parent-route";
import { streamIcsDownload } from "~/lib/ics-download.server";
import {
  createUnauthorizedResponse,
  extractParams,
  isStringParam,
  type RouteContext,
} from "~/lib/route-wrapper-utils.server";

export const runtime = "nodejs";

// Streams a parent-visible appointment as a text/calendar (.ics) download,
// authenticated with the parent-portal session. Wrapped in `withParentAuth`
// (response-aware auth) so a session token refreshed during the download — by
// the outer wrapper or by streamIcsDownload's retry-on-401 — is persisted back
// as a Set-Cookie. Without it the rotated cookie is dropped and a later download
// after the refresh could fail with a stale token.
export const GET = withParentAuth(
  async (request: NextRequest, context: RouteContext): Promise<Response> => {
    const session = await parentAuth();
    if (!session?.user?.token) {
      return createUnauthorizedResponse();
    }
    const params = await extractParams(request, context);
    const appointmentId = params.appointmentId;
    if (!isStringParam(appointmentId) || appointmentId.length === 0) {
      return new Response("Appointment ID is required", { status: 400 });
    }
    return streamIcsDownload(
      `/parent/me/calendar/appointments/${encodeURIComponent(appointmentId)}/ics`,
      session.user.token,
      uncachedParentAuth,
    );
  },
);
