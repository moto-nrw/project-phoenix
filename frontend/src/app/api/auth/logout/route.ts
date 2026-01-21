import { type NextRequest, NextResponse } from "next/server";
import { auth, getCookieHeader } from "~/server/auth";
import { env } from "~/env";

export async function POST(_request: NextRequest) {
  try {
    const session = await auth();

    if (!session?.user) {
      return NextResponse.json({ error: "No active session" }, { status: 401 });
    }

    const cookieHeader = await getCookieHeader();

    // Forward to backend with cookies
    const response = await fetch(`${env.NEXT_PUBLIC_API_URL}/auth/logout`, {
      method: "POST",
      headers: {
        Cookie: cookieHeader,
        "Content-Type": "application/json",
      },
    });

    if (!response.ok && response.status !== 204) {
      const errorText = await response.text();
      console.error(`Logout error: ${response.status}`, errorText);
    }

    // Always return success to client
    return new NextResponse(null, { status: 204 });
  } catch (error) {
    console.error("Logout route error:", error);
    // Still return success - logout should always succeed on client side
    return new NextResponse(null, { status: 204 });
  }
}
