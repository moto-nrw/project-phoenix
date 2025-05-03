import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { env } from "~/env";

const API_URL = env.NEXT_PUBLIC_API_URL;

export async function GET(request: NextRequest) {
  const session = await auth();

  if (!session?.user?.token) {
    return NextResponse.json(
      { error: "Unauthorized: No valid session" },
      { status: 401 },
    );
  }

  // Extract and forward query parameters
  const url = new URL(`${API_URL}/activities/student/available`);
  const searchParams = request.nextUrl.searchParams;

  // Add all search parameters to the request
  Array.from(searchParams.entries()).forEach(([key, value]) => {
    url.searchParams.append(key, value);
  });

  // Check for required student_id parameter
  if (!searchParams.has("student_id")) {
    return NextResponse.json(
      { error: "Missing required parameter: student_id" },
      { status: 400 },
    );
  }

  try {
    const response = await fetch(url.toString(), {
      headers: {
        Authorization: `Bearer ${session.user.token}`,
        "Content-Type": "application/json",
      },
    });

    if (!response.ok) {
      const errorText = await response.text();
      console.error(`API error: ${response.status}`, errorText);
      return NextResponse.json(
        { error: `Backend error: ${response.status}` },
        { status: response.status },
      );
    }

    const data: unknown = await response.json();
    return NextResponse.json(data);
  } catch (error: unknown) {
    console.error("Error fetching available activities:", error);
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
