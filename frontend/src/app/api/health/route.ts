import { NextResponse } from "next/server";

/**
 * Health check endpoint for container orchestration and load balancers.
 * Returns a simple 200 OK response with timestamp.
 *
 * Used by:
 * - Docker health checks (docker-compose.prod.yml)
 * - CI/CD deployment verification
 * - Monitoring systems
 */
export async function GET() {
  return NextResponse.json({
    status: "ok",
    service: "phoenix-frontend",
    timestamp: new Date().toISOString(),
  });
}
