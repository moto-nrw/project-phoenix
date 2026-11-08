/** Client log ingestion from the Träger portal, with its own session. */

import type { NextRequest } from "next/server";
import { traegerAuth, withTraegerAuth } from "~/server/auth/traeger";
import { ingestClientLogs } from "~/lib/client-log-ingest.server";

async function POSTHandler(request: NextRequest) {
  return ingestClientLogs(request, await traegerAuth(), "traeger");
}

export const POST = withTraegerAuth(POSTHandler);
