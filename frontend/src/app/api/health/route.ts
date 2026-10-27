import { NextResponse } from "next/server";
import { withPrivateNoStore } from "~/lib/private-no-store";

/**
 * Health check endpoint for container orchestration and load balancers.
 * Returns a simple 200 OK response with timestamp.
 *
 * Used by:
 * - Docker health checks (docker-compose.prod.yml)
 * - CI/CD deployment verification
 * - Monitoring systems
 */
async function GETHandler() {
  return NextResponse.json({
    status: "ok",
    service: "phoenix-frontend",
    timestamp: new Date().toISOString(),
  });
}

async function HEADHandler() {
  return new Response(null, { status: 200 });
}

export const GET = withPrivateNoStore(GETHandler);
export const HEAD = withPrivateNoStore(HEADHandler);
