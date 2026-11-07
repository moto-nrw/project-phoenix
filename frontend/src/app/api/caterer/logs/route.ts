/**
 * Client-side log ingestion from the caterer portal (ADR 0077), with the
 * caterer session, so its errors reach the logs.
 */

import type { NextRequest } from "next/server";
import { catererAuth, withCatererAuth } from "~/server/auth/caterer";
import { ingestClientLogs } from "~/lib/client-log-ingest.server";

async function POSTHandler(request: NextRequest) {
  return ingestClientLogs(request, await catererAuth(), "caterer");
}

export const POST = withCatererAuth(POSTHandler);
