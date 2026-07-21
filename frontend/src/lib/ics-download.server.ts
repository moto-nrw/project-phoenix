import { getServerApiUrl } from "~/lib/server-api-url";

// A session shape carrying the short-lived backend JWT. Kept minimal so both the
// staff (`uncachedAuth`) and parent (`uncachedParentAuth`) refreshers fit.
type TokenSession = { user?: { token?: string | null } | null } | null;

/**
 * Streams a text/calendar (.ics) download from the backend, refreshing the
 * session token once on a 401 and retrying — mirroring the JSON proxy routes'
 * refresh/retry (`uncachedAuth`) so the download doesn't fail when the 15-minute
 * backend JWT has expired but the session can still refresh.
 */
export async function streamIcsDownload(
  path: string,
  token: string,
  refresh: () => Promise<TokenSession>,
): Promise<Response> {
  const url = `${getServerApiUrl()}${path}`;
  const request = (bearer: string) =>
    fetch(url, { headers: { Authorization: `Bearer ${bearer}` } });

  let upstream = await request(token);
  if (upstream.status === 401) {
    const refreshed = await refresh();
    const newToken = refreshed?.user?.token;
    // Only retry when the token actually changed, matching the JSON wrappers.
    if (newToken && newToken !== token) {
      upstream = await request(newToken);
    }
  }

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
