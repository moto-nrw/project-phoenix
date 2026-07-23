import { type NextRequest } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";

export const runtime = "nodejs";

// Public, unauthenticated iCalendar subscription feed. External calendar apps
// (Apple/Google/Outlook) poll this URL; the token in the path is the only
// credential. Proxies to the backend's /public/calendar/{token} endpoint.
export async function GET(
  _request: NextRequest,
  context: { params: Promise<{ token: string }> },
) {
  const { token } = await context.params;
  const upstream = await fetch(
    `${getServerApiUrl()}/public/calendar/${encodeURIComponent(token)}`,
  );
  if (!upstream.ok) {
    return new Response("Not found", { status: upstream.status });
  }
  return new Response(await upstream.text(), {
    status: 200,
    headers: {
      "Content-Type": "text/calendar; charset=utf-8",
      "Cache-Control": "private, max-age=3600",
    },
  });
}
