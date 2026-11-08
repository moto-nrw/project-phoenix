/** Client log ingestion from the Träger portal, with its own session. */

import type { NextRequest } from "next/server";
import { traegerPortal } from "~/server/auth/traeger";
import { ingestClientLogs } from "~/lib/client-log-ingest.server";

async function POSTHandler(request: NextRequest) {
  return ingestClientLogs(request, await traegerPortal.auth(), "traeger");
}

export const POST = traegerPortal.withAuth(POSTHandler);
