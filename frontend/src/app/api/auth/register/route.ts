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

    // Return the backend response directly
    const responseData = (await response.json()) as Record<string, unknown>;
    return NextResponse.json(responseData, { status: response.status });
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
