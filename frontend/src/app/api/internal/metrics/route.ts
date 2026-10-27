import type { NextRequest } from "next/server";
import { metricsResponse } from "~/lib/server-metrics.server";
import { withPrivateNoStore } from "~/lib/private-no-store";

export const dynamic = "force-dynamic";

function GETHandler(request: NextRequest) {
  return metricsResponse(request);
}

export const GET = withPrivateNoStore(GETHandler);
