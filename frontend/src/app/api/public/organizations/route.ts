import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

/**
 * Public API route to search organizations.
 * Proxies to Better Auth's public organizations endpoint.
 * No authentication required.
 */

function getBetterAuthUrl(): string {
  return process.env.BETTERAUTH_INTERNAL_URL ?? "http://localhost:3001";
}

export async function GET(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url);
    const search = searchParams.get("search") ?? "";
    const limit = searchParams.get("limit") ?? "10";

    const url = new URL(`${getBetterAuthUrl()}/api/auth/public/organizations`);
    if (search) {
      url.searchParams.set("search", search);
    }
    url.searchParams.set("limit", limit);

    const response = await fetch(url.toString(), {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
    });

    if (!response.ok) {
      console.error("Better Auth org search failed:", response.status);
      return NextResponse.json(
        { error: "Failed to search organizations" },
        { status: response.status },
      );
    }

    const organizations = (await response.json()) as Array<{
      id: string;
      name: string;
      slug: string;
    }>;
    return NextResponse.json(organizations);
  } catch (error) {
    console.error("Failed to search organizations:", error);
    return NextResponse.json(
      { error: "Internal server error" },
      { status: 500 },
    );
  }
}
