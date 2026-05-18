import { type NextRequest, NextResponse } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { getClientForwardHeaders } from "~/lib/client-headers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AuthProxy" });

interface ForwardOptions {
  readonly method?: "GET" | "POST" | "PUT" | "DELETE";
  readonly hasBody?: boolean;
}

function buildForwardHeaders(request: NextRequest): Record<string, string> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...getClientForwardHeaders(request),
  };
  const cookieHeader = request.headers.get("cookie");
  if (cookieHeader) headers.Cookie = cookieHeader;
  const authHeader = request.headers.get("authorization");
  if (authHeader) headers.Authorization = authHeader;
  return headers;
}

async function readBackendBody(
  response: Response,
  backendPath: string,
): Promise<unknown> {
  const contentType = response.headers.get("content-type");
  const responseText = await response.text();
  if (!contentType?.includes("application/json")) {
    return { message: responseText || "Request failed with no response" };
  }
  if (!responseText) return null;
  try {
    return JSON.parse(responseText) as unknown;
  } catch (jsonError) {
    logger.error("failed to parse backend JSON", {
      path: backendPath,
      error: jsonError instanceof Error ? jsonError.message : String(jsonError),
    });
    return { message: responseText };
  }
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
      headers: buildForwardHeaders(request),
      body: bodyJson,
    });

    if (response.status === 204) {
      const out = new NextResponse(null, { status: 204 });
      mirrorSetCookies(response, out);
      return out;
    }

    const data = await readBackendBody(response, backendPath);
    const out = NextResponse.json(data ?? { message: "Empty response" }, {
      status: response.status,
    });
    mirrorSetCookies(response, out);
    return out;
  } catch (error) {
    logger.error("proxy_failed", {
      path: backendPath,
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
