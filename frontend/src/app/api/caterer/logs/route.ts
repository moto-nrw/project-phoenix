/**
 * Client-side log ingestion from the caterer portal (ADR 0077), with the
 * caterer session, so its errors reach the logs.
 */

import type { NextRequest } from "next/server";
import { catererPortal } from "~/server/auth/caterer";
import { ingestClientLogs } from "~/lib/client-log-ingest.server";

async function POSTHandler(request: NextRequest) {
  return ingestClientLogs(request, await catererPortal.auth(), "caterer");
}

export const POST = catererPortal.withAuth(POSTHandler);
