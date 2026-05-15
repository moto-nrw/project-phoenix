import { type NextRequest, NextResponse } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { getClientForwardHeaders } from "~/lib/client-headers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AuthProxy" });

interface ForwardOptions {
  readonly method?: "GET" | "POST" | "PUT" | "DELETE";
  readonly hasBody?: boolean;
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

    const cookieHeader = request.headers.get("cookie");
    const authHeader = request.headers.get("authorization");
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      ...getClientForwardHeaders(request),
    };
    if (cookieHeader) headers.Cookie = cookieHeader;
    if (authHeader) headers.Authorization = authHeader;

    const response = await fetch(`${getServerApiUrl()}${backendPath}`, {
      method,
      headers,
      body: bodyJson,
    });

    if (response.status === 204) {
      const out = new NextResponse(null, { status: 204 });
      for (const cookie of response.headers.getSetCookie()) {
        out.headers.append("set-cookie", cookie);
      }
      return out;
    }

    let data: unknown;
    const contentType = response.headers.get("content-type");
    const responseText = await response.text();

    if (contentType?.includes("application/json")) {
      try {
        data = responseText ? (JSON.parse(responseText) as unknown) : null;
      } catch (jsonError) {
        logger.error("failed to parse backend JSON", {
          path: backendPath,
          error:
            jsonError instanceof Error ? jsonError.message : String(jsonError),
        });
        data = { message: responseText };
      }
    } else {
      data = { message: responseText || "Request failed with no response" };
    }

    const out = NextResponse.json(data ?? { message: "Empty response" }, {
      status: response.status,
    });
    for (const cookie of response.headers.getSetCookie()) {
      out.headers.append("set-cookie", cookie);
    }
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
