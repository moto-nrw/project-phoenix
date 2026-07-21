import { type NextRequest } from "next/server";
import { parentAuth, uncachedParentAuth } from "~/server/auth/parent";
import { streamIcsDownload } from "~/lib/ics-download.server";

export const runtime = "nodejs";

// Streams a parent-visible appointment as a text/calendar (.ics) download,
// authenticated with the parent-portal session. Refreshes and retries on a 401
// (expired backend JWT) via streamIcsDownload, like the parent JSON routes.
export async function GET(
  _request: NextRequest,
  context: { params: Promise<{ appointmentId: string }> },
) {
  const session = await parentAuth();
  if (!session?.user?.token) {
    return new Response("Unauthorized", { status: 401 });
  }
  const { appointmentId } = await context.params;
  return streamIcsDownload(
    `/parent/me/calendar/appointments/${encodeURIComponent(appointmentId)}/ics`,
    session.user.token,
    uncachedParentAuth,
  );
}
