import { NextResponse } from "next/server";
import { auth } from "~/server/auth";

// GET handler for fetching avatar images
export const GET = async (
  request: Request,
  { params }: { params: Promise<{ filename: string }> }
) => {
  try {
    const session = await auth();
    if (!session?.user?.token) {
      return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
    }

    // Await params as required by Next.js 15
    const { filename } = await params;
    if (!filename) {
      return NextResponse.json({ error: "Filename required" }, { status: 400 });
    }

    // Fetch from backend
    const backendUrl = `${process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080'}/api/me/profile/avatar/${filename}`;
    const response = await fetch(backendUrl, {
      headers: {
        'Authorization': `Bearer ${session.user.token}`,
      },
    });

    if (!response.ok) {
      return NextResponse.json(
        { error: `Failed to fetch avatar: ${response.status}` },
        { status: response.status }
      );
    }

    // Get the image data and content type
    const contentType = response.headers.get('content-type') || 'image/jpeg';
    const buffer = await response.arrayBuffer();

    // Return the image with proper headers
    return new NextResponse(buffer, {
      headers: {
        'Content-Type': contentType,
        'Cache-Control': 'private, max-age=86400', // Cache for 1 day
      },
    });
  } catch (error) {
    console.error("Avatar fetch error:", error);
    return NextResponse.json(
      { error: "Failed to fetch avatar" },
      { status: 500 }
    );
  }
};