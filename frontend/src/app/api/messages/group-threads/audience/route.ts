import { proxyGet } from "~/lib/route-proxy.server";

/**
 * Proxy GET /api/messages/group-threads/audience → backend (#4784). The
 * children a group, class or AG reaches, with whether a parent can read the
 * message. The query string (target_type, target_id) is forwarded.
 */
export const GET = proxyGet(() => "/api/messages/group-threads/audience");
