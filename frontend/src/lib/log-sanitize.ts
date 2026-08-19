/**
 * Strips a URL or endpoint down to a log-safe form: the query string is
 * removed entirely (staff-UI searches carry student names and e-mail
 * addresses as query parameters — issue #2105) and path segments that look
 * like numeric IDs, UUIDs, or tokens are masked. Browser logs are shipped to
 * /api/logs and land in Loki, so this applies to client and server logging
 * alike.
 */
export function sanitizeEndpoint(endpoint: string): string {
  const [path = ""] = endpoint.split("?");
  return path
    .split("/")
    .map((segment) => {
      if (!segment) return segment;
      if (/^\d+$/.test(segment)) return "{id}";
      if (/^[0-9a-f]{8}-[0-9a-f-]{27,}$/i.test(segment)) return "{uuid}";
      if (/^[A-Za-z0-9_-]{16,}$/.test(segment)) return "{token}";
      return segment;
    })
    .join("/");
}
