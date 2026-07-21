import { type NextRequest } from "next/server";
import { auth, uncachedAuth } from "~/server/auth";
import { streamIcsDownload } from "~/lib/ics-download.server";

export const runtime = "nodejs";

// Streams the appointment as a text/calendar (.ics) download. Kept out of the
// JSON route-wrapper because the body is an iCalendar document, not JSON, but it
// reuses the same refresh/retry-on-401 behaviour via streamIcsDownload.
export async function GET(
  _request: NextRequest,
  context: { params: Promise<{ appointmentId: string }> },
) {
  const session = await auth();
  if (!session?.user?.token) {
    return new Response("Unauthorized", { status: 401 });
  }
  const { appointmentId } = await context.params;
  return streamIcsDownload(
    `/api/calendar/appointments/${encodeURIComponent(appointmentId)}/ics`,
    session.user.token,
    uncachedAuth,
  );
}
