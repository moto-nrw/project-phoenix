// Test endpoint to debug groups API
import { apiGet } from "~/lib/api-helpers";
import { auth } from "~/server/auth";
import { NextResponse } from "next/server";

export async function GET() {
  try {
    // Get the auth session
    const session = await auth();
    console.log("Test: Auth session:", session);

    if (!session?.user?.token) {
      return NextResponse.json(
        { error: "No auth token found" },
        { status: 401 },
      );
    }

    // Try to get groups directly from the backend
    const endpoint = `/api/groups`;
    const response = await apiGet(endpoint, session.user.token);

    console.log("Test: Direct backend response:", response);

    return NextResponse.json({
      session: {
        userId: session.user.id,
        email: session.user.email,
        hasToken: !!session.user.token,
      },
      backendResponse: response,
    });
  } catch (error) {
    console.error("Test: Error fetching groups:", error);
    const errorMessage =
      error instanceof Error ? error.message : "Unknown error";
    const errorDetails =
      error instanceof Error && "response" in error
        ? ((error as unknown as { response?: { data?: unknown } })?.response
            ?.data ?? error)
        : error;

    return NextResponse.json(
      {
        error: errorMessage,
        details: errorDetails,
      },
      { status: 500 },
    );
  }
}
