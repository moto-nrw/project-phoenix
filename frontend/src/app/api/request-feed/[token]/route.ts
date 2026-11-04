import { type NextRequest } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { clientIdentityHeaders } from "~/lib/client-headers.server";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { withPrivateNoStore } from "~/lib/private-no-store";

export const runtime = "nodejs";

// Public RSS readers use this tenant-host URL. The path secret is the only
// credential and is forwarded to the backend without entering a user session.
async function GETHandler(
  _request: NextRequest,
  context: { params: Promise<{ token: string }> },
) {
  const { token } = await context.params;
  const upstream = await fetch(
    `${getServerApiUrl()}/public/request-feed/${encodeURIComponent(token)}`,
    { headers: clientIdentityHeaders(_request.headers), cache: "no-store" },
  );
  if (!upstream.ok) {
    const response = forwardBackendResponse(upstream, _request);
    response.headers.set("Cache-Control", "private, no-store");
    return response;
  }
  return new Response(await upstream.text(), {
    status: 200,
    headers: {
      "Content-Type": "application/rss+xml; charset=utf-8",
      "Cache-Control": "private, no-store",
    },
  });
}

export const GET = withPrivateNoStore(GETHandler);
