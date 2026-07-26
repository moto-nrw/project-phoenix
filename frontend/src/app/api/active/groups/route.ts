// app/api/active/groups/route.ts
import { proxyGet, proxyPost } from "~/lib/route-proxy.server";

/**
 * Type definition for group creation request
 */
interface GroupCreateRequest {
  name: string;
  description?: string;
  room_id?: string;
}

/**
 * Handler for GET /api/active/groups
 * Returns a list of active groups, optionally filtered by query parameters
 */
export const GET = proxyGet("/api/active/groups", { raw: true });

/**
 * Handler for POST /api/active/groups
 * Creates a new active group
 */
export const POST = proxyPost<unknown, GroupCreateRequest>(
  "/api/active/groups",
  {
    raw: true,
  },
);
