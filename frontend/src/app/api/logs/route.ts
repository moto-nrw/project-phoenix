/**
 * API endpoint for client-side log ingestion from the tenant (staff) portal.
 * The parents portal ships to /api/parent/logs with its own session.
 */

import type { NextRequest } from "next/server";
import { auth, withTenantAuth } from "~/server/auth";
import { ingestClientLogs } from "~/lib/client-log-ingest.server";

async function POSTHandler(request: NextRequest) {
  return ingestClientLogs(request, await auth(), "tenant");
}

export const POST = withTenantAuth(POSTHandler);
