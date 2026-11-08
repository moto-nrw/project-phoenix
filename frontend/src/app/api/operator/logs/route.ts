/** Client log ingestion from the operator portal, with its own session. */

import type { NextRequest } from "next/server";
import { operatorPortal } from "~/server/auth/operator";
import { ingestClientLogs } from "~/lib/client-log-ingest.server";

async function POSTHandler(request: NextRequest) {
  return ingestClientLogs(request, await operatorPortal.auth(), "operator");
}

export const POST = operatorPortal.withAuth(POSTHandler);
