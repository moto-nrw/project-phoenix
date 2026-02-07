// Settings tab by key API route
// Note: Uses direct passthrough to avoid double-wrapping the backend response
import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { env } from "~/env";

export async function GET(
  request: NextRequest,
  context: { params: Promise<{ tab: string }> },
) {
  try {
    const session = await auth();
    if (!session?.user?.token) {
      return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
    }

    const { tab } = await context.params;
    if (!tab) {
      return NextResponse.json(
        { error: "Tab parameter required" },
        { status: 400 },
      );
    }

    // Forward query params
    const queryParams = new URLSearchParams();
    request.nextUrl.searchParams.forEach((value, key) => {
      queryParams.append(key, value);
    });
    const queryString = queryParams.toString();
    const endpoint = `/api/settings/tabs/${tab}${queryString ? `?${queryString}` : ""}`;

    const response = await fetch(`${env.NEXT_PUBLIC_API_URL}${endpoint}`, {
      headers: {
        Authorization: `Bearer ${session.user.token}`,
      },
    });

    if (!response.ok) {
      const errorText = await response.text();
      return NextResponse.json(
        { error: errorText },
        { status: response.status },
      );
    }

    // Pass through backend response directly without re-wrapping
    const data: unknown = await response.json();
    return NextResponse.json(data);
  } catch (error) {
    console.error("Error fetching tab settings:", error);
    return NextResponse.json(
      { error: "Internal server error" },
      { status: 500 },
    );
  }
}
