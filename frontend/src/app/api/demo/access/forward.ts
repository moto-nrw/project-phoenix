import { type NextRequest, NextResponse } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { getClientForwardHeaders } from "~/lib/client-headers.server";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "DemoAccessRoute" });

// Forwards a demo token to the backend (#3462). The token comes in the POST
// body and is never logged; the backend answer passes through unchanged.
export async function forwardDemoToken(
  request: NextRequest,
  send: (token: string, headers: Record<string, string>) => Promise<Response>,
): Promise<NextResponse> {
  let token = "";
  try {
    const body = (await request.json()) as { token?: unknown };
    if (typeof body.token === "string") token = body.token;
  } catch {
    // An unreadable body is an unknown token.
  }
  if (!token) {
    return NextResponse.json(
      { error: "demo access is unknown" },
      { status: 404 },
    );
  }
  try {
    const response = await send(token, getClientForwardHeaders(request));
    const payload: unknown = await response.json().catch(() => ({}));
    return NextResponse.json(payload, { status: response.status });
  } catch (error) {
    logger.error("demo_access_forward_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

export function demoBackendUrl(path: string): string {
  return `${getServerApiUrl()}/demo/access/${path}`;
}
