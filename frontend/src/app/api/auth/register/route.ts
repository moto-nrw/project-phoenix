import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { getServerApiUrl } from "~/lib/server-api-url";

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

    // Log the response status for debugging
    console.log(`Registration response status: ${response.status}`);

    // Check if the response has a body and is JSON
    let responseData: Record<string, unknown> | null = null;
    const contentType = response.headers.get("content-type");

    if (contentType?.includes("application/json")) {
      try {
        responseData = (await response.json()) as Record<string, unknown>;
      } catch (jsonError) {
        console.error("Failed to parse JSON response:", jsonError);
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

    // Log the actual response for debugging
    if (!response.ok) {
      console.error("Registration failed:", {
        status: response.status,
        contentType: contentType,
      });
    }

    return NextResponse.json(
      responseData || { status: "error", error: "Empty response" },
      { status: response.status },
    );
  } catch (error) {
    console.error("Registration error:", error);
    return NextResponse.json(
      {
        message: "An error occurred during registration",
        error: String(error),
      },
      { status: 500 },
    );
  }
}
