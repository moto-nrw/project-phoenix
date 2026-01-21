import { NextResponse } from "next/server";
import { auth } from "~/server/auth";

/**
 * This endpoint is used to check the current session status.
 * BetterAuth uses cookies for session management, so no explicit refresh is needed.
 */
export async function GET() {
  try {
    const session = await auth();

    if (!session) {
      return NextResponse.json({ error: "No session found" }, { status: 401 });
    }

    // Return the current session state
    // BetterAuth uses cookies, so we just check if user exists
    return NextResponse.json({
      success: true,
      hasSession: !!session.user,
    });
  } catch (error) {
    console.error("Error checking session:", error);
    return NextResponse.json(
      { error: "Failed to check session" },
      { status: 500 },
    );
  }
}
