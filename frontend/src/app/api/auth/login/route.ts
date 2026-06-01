import { type NextRequest, NextResponse } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { getClientForwardHeaders } from "~/lib/client-headers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AuthLoginRoute" });

export async function POST(request: NextRequest) {
  try {
    const body: unknown = await request.json();

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

    let data: unknown;
    const contentType = response.headers.get("content-type");
    const responseText = await response.text();

    if (contentType?.includes("application/json")) {
      try {
        data = responseText ? (JSON.parse(responseText) as unknown) : null;
      } catch (jsonError) {
        logger.error("failed to parse JSON response", {
          error:
            jsonError instanceof Error ? jsonError.message : String(jsonError),
        });
        data = {
          message: responseText,
        };
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
    logger.error("login failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
