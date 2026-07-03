// app/api/rooms/by-category/route.ts
import { proxyGet } from "~/lib/route-proxy.server";
import type { BackendRoom } from "~/lib/room-helpers";

/**
 * Handler for GET /api/rooms/by-category
 * Returns rooms grouped by their category
 */
export const GET = proxyGet<Record<string, BackendRoom[]>>(
  "/api/rooms/by-category",
  { raw: true },
);
