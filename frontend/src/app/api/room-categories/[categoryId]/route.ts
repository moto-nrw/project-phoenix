import { proxyDelete, proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface RoomCategoryBody {
  name: string;
  color: string;
}

/**
 * PUT /api/room-categories/{categoryId}
 * Rename or recolor a room category; its rooms move with a new name.
 */
export const PUT = proxyPut<unknown, RoomCategoryBody>(
  (p) => `/api/room-categories/${requirePathSegmentParam(p, "categoryId")}`,
);

/**
 * DELETE /api/room-categories/{categoryId}
 * Delete a room category no room uses any more.
 */
export const DELETE = proxyDelete(
  (p) => `/api/room-categories/${requirePathSegmentParam(p, "categoryId")}`,
);
