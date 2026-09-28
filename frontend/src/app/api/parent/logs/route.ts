/**
 * API endpoint for client-side log ingestion from the parents portal.
 *
 * Authenticates with the parent session (parent.session-token) and verifies
 * it as a parent-scope token, so errors in the parents portal reach the logs
 * instead of being rejected by the tenant-only /api/logs route.
 */

import type { NextRequest } from "next/server";
import { parentAuth } from "~/server/auth/parent";
import { withParentAuth } from "~/server/auth/parent-route";
import { ingestClientLogs } from "~/lib/client-log-ingest.server";

async function POSTHandler(request: NextRequest) {
  return ingestClientLogs(request, await parentAuth(), "parent");
}

export const POST = withParentAuth(POSTHandler);
