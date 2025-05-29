import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { env } from "~/env";

export async function POST(request: NextRequest) {
  try {
    // Forward the registration request to the backend
    const requestBody = (await request.json()) as Record<string, unknown>;

    console.log(
      `Forwarding registration request to ${env.NEXT_PUBLIC_API_URL}/auth/register`,
      requestBody,
    );

    const response = await fetch(`${env.NEXT_PUBLIC_API_URL}/auth/register`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(requestBody),
    });

    // Log the response status for debugging
    console.log(`Registration response status: ${response.status}`);

    // Check if the response has a body and is JSON
    let responseData: Record<string, unknown> | null = null;
    const contentType = response.headers.get("content-type");

    if (contentType?.includes("application/json")) {
      try {
        responseData = await response.json() as Record<string, unknown>;
      } catch (jsonError) {
        console.error("Failed to parse JSON response:", jsonError);
        responseData = {
          status: "error",
          error: await response.text() || "Failed to parse response"
        };
      }
    } else {
      // If not JSON, get the text response
      const text = await response.text();
      responseData = {
        status: "error",
        error: text || "Request failed with no response"
      };
    }

    // Log the actual response for debugging
    if (!response.ok) {
      console.error("Registration failed:", {
        status: response.status,
        contentType: contentType,
        responseData: responseData,
      });
    }

    return NextResponse.json(responseData || { status: "error", error: "Empty response" }, { status: response.status });
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