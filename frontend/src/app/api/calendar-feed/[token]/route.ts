import { type NextRequest } from "next/server";
import { handleApiError } from "~/lib/api-response-error.server";
import { sendBackend } from "~/lib/backend-client.server";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { withPrivateNoStore } from "~/lib/private-no-store";

export const runtime = "nodejs";

// Public, unauthenticated iCalendar subscription feed. External calendar apps
// (Apple/Google/Outlook) poll this URL; the token in the path is the only
// credential. Proxies to the backend's /public/calendar/{token} endpoint.
async function GETHandler(
  _request: NextRequest,
  context: { params: Promise<{ token: string }> },
) {
  const { token } = await context.params;
  let upstream: Response;
  try {
    upstream = await sendBackend(
      `/public/calendar/${encodeURIComponent(token)}`,
      null,
      { request: _request },
    );
  } catch (error) {
    return handleApiError(error, _request);
  }
  if (!upstream.ok) {
    return forwardBackendResponse(upstream, _request);
  }
  return new Response(await upstream.text(), {
    status: 200,
    headers: {
      "Content-Type": "text/calendar; charset=utf-8",
      "Cache-Control": "private, max-age=3600",
    },
  });
}

export const GET = withPrivateNoStore(GETHandler);
