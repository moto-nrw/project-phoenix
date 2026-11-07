import { getServerApiUrl } from "~/lib/server-api-url";
import { incomingClientForwardHeaders } from "~/lib/client-headers.server";
import {
  sendWithTokenRefresh,
  type FreshSessionReader,
} from "~/lib/token-refresh-retry.server";

/**
 * Streams a text/calendar (.ics) download from the backend, refreshing the
 * session token once on a 401 and retrying — mirroring the JSON proxy routes'
 * refresh/retry (`uncachedAuth`) so the download doesn't fail when the 15-minute
 * backend JWT has expired but the session can still refresh.
 */
export async function streamIcsDownload(
  path: string,
  token: string,
  refresh: FreshSessionReader,
): Promise<Response> {
  const url = `${getServerApiUrl()}${path}`;
  const forwardHeaders = await incomingClientForwardHeaders();
  const request = (bearer: string) =>
    fetch(url, {
      headers: { Authorization: `Bearer ${bearer}`, ...forwardHeaders },
    });

  const upstream = await sendWithTokenRefresh(token, request, refresh);

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
