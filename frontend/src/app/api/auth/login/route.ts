import { captureBffException } from "~/lib/sentry-bff.server";
import type { NextRequest } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { getClientForwardHeaders } from "~/lib/client-headers.server";
import { createLogger } from "~/lib/logger";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { problemResponse } from "~/lib/api-helpers.server";

const logger = createLogger({ component: "AuthLoginRoute" });

export async function POST(request: NextRequest) {
  try {
    let body: unknown;
    try {
      body = await request.json();
    } catch {
      return problemResponse(400, "Invalid JSON", request);
    }

    const cookieHeader = request.headers.get("cookie");
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      ...getClientForwardHeaders(request),
    };
    if (cookieHeader) headers.Cookie = cookieHeader;

    const response = await fetch(`${getServerApiUrl()}/auth/login`, {
      method: "POST",
      headers,
      body: JSON.stringify(body),
    });

    const out = forwardBackendResponse(response, request);
    for (const cookie of response.headers.getSetCookie()) {
      out.headers.append("set-cookie", cookie);
    }
    return out;
  } catch (error) {
    captureBffException(error, request);
    logger.error("login proxy failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return problemResponse(500, "Internal Server Error", request);
  }
}
