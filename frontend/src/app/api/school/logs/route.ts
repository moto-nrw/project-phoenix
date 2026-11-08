/** Client log ingestion from the school portal, with its own session. */

import type { NextRequest } from "next/server";
import { schoolPortal } from "~/server/auth/school";
import { ingestClientLogs } from "~/lib/client-log-ingest.server";

async function POSTHandler(request: NextRequest) {
  return ingestClientLogs(request, await schoolPortal.auth(), "school");
}

export const POST = schoolPortal.withAuth(POSTHandler);
