import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AuthRegisterRoute" });

export async function POST(request: NextRequest) {
  try {
    // Forward the registration request to the backend
    const requestBody = (await request.json()) as Record<string, unknown>;

    // Get session to forward authentication if available
    const session = await auth();

    // Prepare headers - include Authorization if authenticated
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
    };

    // If authenticated, forward the access token for admin role validation
    if (session?.user?.token) {
      headers.Authorization = `Bearer ${session.user.token}`;
    }

    const response = await fetch(`${getServerApiUrl()}/auth/register`, {
      method: "POST",
      headers,
      body: JSON.stringify(requestBody),
    });

    logger.debug("registration response received", { status: response.status });

    // Check if the response has a body and is JSON
    let responseData: Record<string, unknown> | null = null;
    const contentType = response.headers.get("content-type");

    if (contentType?.includes("application/json")) {
      try {
        responseData = (await response.json()) as Record<string, unknown>;
      } catch (jsonError) {
        logger.error("failed to parse JSON response", {
          error:
            jsonError instanceof Error ? jsonError.message : String(jsonError),
        });
        responseData = {
          status: "error",
          error: (await response.text()) || "Failed to parse response",
        };
      }
    } else {
      // If not JSON, get the text response
      const text = await response.text();
      responseData = {
        status: "error",
        error: text || "Request failed with no response",
      };
    }

    if (!response.ok) {
      logger.error("registration failed", {
        status: response.status,
        content_type: contentType,
      });
    }

    return NextResponse.json(
      responseData || { status: "error", error: "Empty response" },
      { status: response.status },
    );
  } catch (error) {
    logger.error("registration error", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      {
        message: "An error occurred during registration",
        error: String(error),
      },
      { status: 500 },
    );
  }
}
