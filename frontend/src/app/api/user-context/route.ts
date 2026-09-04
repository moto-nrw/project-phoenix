// app/api/user-context/route.ts
// BFF (Backend-for-Frontend) endpoint for shared user context data
// Consolidates 3 API calls into 1 to eliminate redundant auth() overhead
import type { NextRequest } from "next/server";
import { createGetHandler } from "~/lib/route-wrapper.server";
import type { UserContextResponse } from "~/lib/user-context-types";
import { loadUserContext } from "~/lib/user-context.server";

/**
 * GET /api/user-context
 *
 * BFF endpoint that fetches user context data needed across multiple pages.
 * The Go backend returns one complete projection, so one browser request maps
 * to one backend request and partial access data cannot be hidden as empty.
 *
 * Used by: /students/search, and potentially other pages that need user context
 * The tenant layout preloads the same projection on the server (#2973); this
 * route serves later revalidations and sessions without a preload.
 */
export const GET = createGetHandler<UserContextResponse>(
  async (_request: NextRequest, token: string) => loadUserContext(token),
);
