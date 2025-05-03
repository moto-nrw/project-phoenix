import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { env } from "~/env";

export async function GET(request: NextRequest) {
  // Get authentication session
  const session = await auth();

  if (!session?.user?.token) {
    return NextResponse.json(
      { error: "Unauthorized: No valid session" },
      { status: 401 },
    );
  }

  // Parse query parameters
  const searchParams = request.nextUrl.searchParams;
  const search = searchParams.get("search");

  // Build backend API URL with parameters
  const url = new URL(`${env.NEXT_PUBLIC_API_URL}/groups/public`);
  if (search) url.searchParams.append("search", search);

  try {
    // Check if user has proper roles
    if (!session.user.roles || session.user.roles.length === 0) {
      console.warn("User has no roles for API request");
    }

    console.log("Making API request with roles:", session.user.roles);

    // Forward the request to the backend with token
    const backendResponse = await fetch(url.toString(), {
      headers: {
        Authorization: `Bearer ${session.user.token}`,
        "Content-Type": "application/json",
      },
    });

    if (!backendResponse.ok) {
      const errorText = await backendResponse.text();
      console.error(`Backend API error: ${backendResponse.status}`, errorText);
      return NextResponse.json(
        { error: `Backend error: ${backendResponse.status}` },
        { status: backendResponse.status },
      );
    }

    const data: unknown = await backendResponse.json();
    return NextResponse.json(data);
  } catch (error: unknown) {
    console.error("Error fetching public groups:", error);
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
