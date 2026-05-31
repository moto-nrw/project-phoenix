import type { NextRequest } from "next/server";
import { metricsResponse } from "~/lib/server-metrics";

export const dynamic = "force-dynamic";

export function GET(request: NextRequest) {
  return metricsResponse(request);
}
