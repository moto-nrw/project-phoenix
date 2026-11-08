/** Client log ingestion from the school portal, with its own session. */

import type { NextRequest } from "next/server";
import { schoolAuth, withSchoolAuth } from "~/server/auth/school";
import { ingestClientLogs } from "~/lib/client-log-ingest.server";

async function POSTHandler(request: NextRequest) {
  return ingestClientLogs(request, await schoolAuth(), "school");
}

export const POST = withSchoolAuth(POSTHandler);
