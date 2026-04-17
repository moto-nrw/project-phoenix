import { type NextRequest, NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ActiveSessionsStartRoute" });

/**
 * POST /api/active/sessions/start
 *
 * Proxies web-initiated activity session starts to the backend. Unlike the
 * generic POST wrapper this handler forwards the backend response status and
 * body verbatim so the 409 conflict payload (which carries the conflicting
 * room + active group IDs) reaches the client unchanged. That lets the UI
 * show a confirmation modal and offer a force retry.
 */
export async function POST(request: NextRequest): Promise<NextResponse> {
  const session = await auth();
  const token = session?.user?.token;
  if (!token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const body = (await request.json()) as unknown;

  const { getServerApiUrl } = await import("~/lib/server-api-url");
  const backendURL = `${getServerApiUrl()}/api/active/sessions/start`;

  const backendResponse = await fetch(backendURL, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify(body),
  });

  const responseText = await backendResponse.text();
  let responseJson: unknown = null;
  if (responseText.length > 0) {
    try {
      responseJson = JSON.parse(responseText);
    } catch {
      responseJson = { error: responseText };
    }
  }

  if (!backendResponse.ok && backendResponse.status >= 500) {
    logger.error("session_start_backend_error", {
      status: backendResponse.status,
      body: responseText.slice(0, 500),
    });
  }

  return NextResponse.json(responseJson, { status: backendResponse.status });
}
