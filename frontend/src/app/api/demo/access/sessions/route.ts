import { type NextRequest, NextResponse } from "next/server";
import { getClientForwardHeaders } from "~/lib/client-headers.server";
import { createLogger } from "~/lib/logger";
import {
  DEMO_TOKEN_COOKIE,
  DEMO_TOKEN_COOKIE_OPTIONS,
  demoBackendUrl,
} from "../forward";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";

const logger = createLogger({ component: "DemoSessionRoute" });

// Redeems a demo token in a demo role (#3462, #3467). The entry page sends
// the token from the link; the route keeps it in an httpOnly cookie, and a
// later role switch from the banner sends only the role. The token is never
// logged and never returned.
export async function POST(request: NextRequest) {
  let body: { token?: unknown; role?: unknown } = {};
  try {
    body = (await request.json()) as typeof body;
  } catch {
    // An unreadable body is a switch without a role.
  }
  const linkToken = typeof body.token === "string" ? body.token.trim() : "";
  const token = linkToken || request.cookies.get(DEMO_TOKEN_COOKIE)?.value;
  if (!token) {
    return NextResponse.json(
      { error: "demo access is unknown" },
      { status: 404 },
    );
  }
  const role = typeof body.role === "string" ? body.role : undefined;
  try {
    const backend = await fetch(demoBackendUrl("sessions"), {
      method: "POST",
      headers: {
        ...getClientForwardHeaders(request),
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ token, role }),
    });
    if (!backend.ok) return forwardBackendResponse(backend);
    const payload: unknown = await backend.json().catch(() => ({}));
    const response = NextResponse.json(payload, { status: backend.status });
    if (backend.ok && linkToken) {
      response.cookies.set(
        DEMO_TOKEN_COOKIE,
        linkToken,
        DEMO_TOKEN_COOKIE_OPTIONS,
      );
    }
    return response;
  } catch (error) {
    logger.error("demo_session_forward_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
