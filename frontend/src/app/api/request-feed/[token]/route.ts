import { type NextRequest } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";

export const runtime = "nodejs";

// Public RSS readers use this tenant-host URL. The path secret is the only
// credential and is forwarded to the backend without entering a user session.
export async function GET(
  _request: NextRequest,
  context: { params: Promise<{ token: string }> },
) {
  const { token } = await context.params;
  const upstream = await fetch(
    `${getServerApiUrl()}/public/request-feed/${encodeURIComponent(token)}`,
    { cache: "no-store" },
  );
  if (!upstream.ok) {
    return new Response("Not found", {
      status: upstream.status,
      headers: { "Cache-Control": "private, no-store" },
    });
  }
  return new Response(await upstream.text(), {
    status: 200,
    headers: {
      "Content-Type": "application/rss+xml; charset=utf-8",
      "Cache-Control": "private, no-store",
    },
  });
}
