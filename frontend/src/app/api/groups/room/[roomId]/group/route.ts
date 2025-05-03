import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { env } from "~/env";

export async function GET(
  request: NextRequest,
  { params }: { params: { roomId: string } },
) {
  // Get authentication session
  const session = await auth();

  if (!session?.user?.token) {
    return NextResponse.json(
      { error: "Unauthorized: No valid session" },
      { status: 401 },
    );
  }

  // Make sure params is fully resolved
  const resolvedParams =
    params instanceof Promise
      ? ((await params) as { roomId: string })
      : (params as { roomId: string });
  const roomId: string = resolvedParams.roomId;

  try {
    // Check if user has proper roles
    if (!session.user.roles || session.user.roles.length === 0) {
      console.warn("User has no roles for API request");
    }

    console.log("Making API request with roles:", session.user.roles);

    // Forward the request to the backend with token
    const backendResponse = await fetch(
      `${env.NEXT_PUBLIC_API_URL}/groups/room/${roomId}/group`,
      {
        headers: {
          Authorization: `Bearer ${session.user.token}`,
          "Content-Type": "application/json",
        },
      },
    );

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
    console.error(`Error fetching group for room ${roomId}:`, error);
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
