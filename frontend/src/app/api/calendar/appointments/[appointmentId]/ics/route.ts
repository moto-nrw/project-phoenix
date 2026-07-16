import { type NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { getServerApiUrl } from "~/lib/server-api-url";

export const runtime = "nodejs";

// Streams the appointment as a text/calendar (.ics) download. Kept out of the
// JSON route-wrapper because the body is an iCalendar document, not JSON.
export async function GET(
  _request: NextRequest,
  context: { params: Promise<{ appointmentId: string }> },
) {
  const session = await auth();
  if (!session?.user?.token) {
    return new Response("Unauthorized", { status: 401 });
  }
  const { appointmentId } = await context.params;
  const upstream = await fetch(
    `${getServerApiUrl()}/api/calendar/appointments/${encodeURIComponent(appointmentId)}/ics`,
    { headers: { Authorization: `Bearer ${session.user.token}` } },
  );
  if (!upstream.ok) {
    return new Response("Kalendereintrag nicht verfügbar", {
      status: upstream.status,
    });
  }
  return new Response(await upstream.text(), {
    status: 200,
    headers: {
      "Content-Type": "text/calendar; charset=utf-8",
      "Content-Disposition":
        upstream.headers.get("content-disposition") ??
        'attachment; filename="termin.ics"',
    },
  });
}
