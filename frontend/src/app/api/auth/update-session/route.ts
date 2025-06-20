import { NextResponse } from "next/server";
import { auth } from "~/server/auth";

/**
 * This endpoint is used to trigger a session update
 * It forces NextAuth to re-run the JWT callback which will refresh the token if needed
 */
export async function GET() {
  try {
    const session = await auth();
    
    if (!session) {
      return NextResponse.json(
        { error: "No session found" },
        { status: 401 }
      );
    }
    
    // The session should have been updated by the JWT callback
    // Return the current session state
    return NextResponse.json({
      success: true,
      hasToken: !!session.user?.token,
      tokenError: session.error,
    });
  } catch (error) {
    console.error("Error updating session:", error);
    return NextResponse.json(
      { error: "Failed to update session" },
      { status: 500 }
    );
  }
}