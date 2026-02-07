import { type NextRequest, NextResponse } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AuthLoginRoute" });

export async function POST(request: NextRequest) {
  try {
    const body: unknown = await request.json();

    const response = await fetch(`${getServerApiUrl()}/auth/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });

    // Check if the response has a body and is JSON
    let data: unknown;
    const contentType = response.headers.get("content-type");

    if (contentType?.includes("application/json")) {
      try {
        data = await response.json();
      } catch (jsonError) {
        logger.error("failed to parse JSON response", {
          error:
            jsonError instanceof Error ? jsonError.message : String(jsonError),
        });
        data = { message: await response.text() };
      }
    } else {
      // If not JSON, get the text response
      const text = await response.text();
      data = { message: text ?? "Request failed with no response" };
    }

    return NextResponse.json(data ?? { message: "Empty response" }, {
      status: response.status,
    });
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
