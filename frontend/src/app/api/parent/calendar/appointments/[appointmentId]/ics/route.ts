import { type NextRequest } from "next/server";
import { parentAuth } from "~/server/auth/parent";
import { getServerApiUrl } from "~/lib/server-api-url";

export const runtime = "nodejs";

// Streams a parent-visible appointment as a text/calendar (.ics) download,
// authenticated with the parent-portal session.
export async function GET(
  _request: NextRequest,
  context: { params: Promise<{ appointmentId: string }> },
) {
  const session = await parentAuth();
  if (!session?.user?.token) {
    return new Response("Unauthorized", { status: 401 });
  }
  const { appointmentId } = await context.params;
  const upstream = await fetch(
    `${getServerApiUrl()}/parent/me/calendar/appointments/${encodeURIComponent(appointmentId)}/ics`,
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
