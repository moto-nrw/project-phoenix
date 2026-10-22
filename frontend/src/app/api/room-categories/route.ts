import { proxyGet, proxyPost } from "~/lib/route-proxy.server";

interface RoomCategoryBody {
  name: string;
  color: string;
}

/**
 * GET /api/room-categories
 * The school's room categories with their badge color (#4352).
 */
export const GET = proxyGet("/api/room-categories");

/**
 * POST /api/room-categories
 * Create a room category.
 */
export const POST = proxyPost<unknown, RoomCategoryBody>(
  "/api/room-categories",
);
