import { type NextRequest } from "next/server";
import { schoolAuth } from "~/server/auth/school";
import { withSchoolAuth } from "~/server/auth/school-route";
import { createLogger } from "~/lib/logger";
import { proxySSEStream } from "~/lib/sse-proxy.server";

const logger = createLogger({ component: "SchoolSSEEventsRoute" });

// REQUIRED for streaming - must use Node.js runtime
export const runtime = "nodejs";

/**
 * School-portal SSE proxy (#2208). Same streaming proxy as the tenant and
 * parent routes, authenticated with the school NextAuth session and proxied to
 * the backend's school-scoped stream at /school-sse/events, which delivers
 * only account-addressed triggers (today: staff_message for the Team-Chat).
 */
async function GETHandler(request: NextRequest) {
  const session = await schoolAuth();

  if (!session?.user?.token) {
    return new Response("Unauthorized", { status: 401 });
  }

  return proxySSEStream(request, {
    token: session.user.token,
    upstreamPath: "/school-sse/events",
    logger,
  });
}

export const GET = withSchoolAuth(GETHandler);
