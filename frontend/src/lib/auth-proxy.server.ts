import { type NextRequest, NextResponse } from "next/server";
import { problemResponse } from "~/lib/api-response-error.server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { getClientForwardHeaders } from "~/lib/client-headers.server";
import { createLogger } from "~/lib/logger";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { captureBffException } from "~/lib/sentry-bff.server";

const logger = createLogger({ component: "AuthProxy" });

interface ForwardOptions {
  readonly method?: "GET" | "POST" | "PUT" | "DELETE";
  readonly hasBody?: boolean;
  /**
   * Routes that act for a signed-in account pass the access token of their
   * portal's session here; the caller's Authorization header is then
   * ignored. The browser holds no session token (#5005). Routes before a
   * session (login, MFA challenge, enrollment) leave it out and forward the
   * caller's header as before.
   */
  readonly sessionToken?: string | null;
}

function buildForwardHeaders(
  request: NextRequest,
  options: ForwardOptions,
): Record<string, string> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...getClientForwardHeaders(request),
  };
  const cookieHeader = request.headers.get("cookie");
  if (cookieHeader) headers.Cookie = cookieHeader;
  const authHeader =
    options.sessionToken === undefined
      ? request.headers.get("authorization")
      : options.sessionToken && `Bearer ${options.sessionToken}`;
  if (authHeader) headers.Authorization = authHeader;
  return headers;
}

function mirrorSetCookies(from: Response, to: NextResponse): void {
  for (const cookie of from.headers.getSetCookie()) {
    to.headers.append("set-cookie", cookie);
  }
}

export async function forwardJsonPost(
  request: NextRequest,
  backendPath: string,
  options: ForwardOptions = {},
): Promise<NextResponse> {
  const method = options.method ?? "POST";
  const hasBody = options.hasBody ?? method === "POST";

  try {
    let bodyJson: string | undefined;
    if (hasBody) {
      const body: unknown = await request.json().catch(() => ({}));
      bodyJson = JSON.stringify(body ?? {});
    }

    const response = await fetch(`${getServerApiUrl()}${backendPath}`, {
      method,
      headers: buildForwardHeaders(request, options),
      body: bodyJson,
    });

    const out = forwardBackendResponse(response, request);
    mirrorSetCookies(response, out);
    return out;
  } catch (error) {
    captureBffException(error, request);
    logger.error("proxy_failed", {
      path: backendPath,
      error: error instanceof Error ? error.message : String(error),
    });
    return problemResponse(500, "Internal Server Error", request);
  }
}
