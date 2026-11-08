/**
 * Client log ingestion from the tenant (staff) portal. Every other portal
 * ships to its own route with its own session (`portalLogPath`).
 */

import type { NextRequest } from "next/server";
import { tenantPortal } from "~/server/auth";
import { ingestClientLogs } from "~/lib/client-log-ingest.server";

async function POSTHandler(request: NextRequest) {
  return ingestClientLogs(request, await tenantPortal.auth(), "tenant");
}

export const POST = tenantPortal.withAuth(POSTHandler);
