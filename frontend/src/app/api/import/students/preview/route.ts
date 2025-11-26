import { NextResponse, type NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { env } from "~/env";

export async function POST(request: NextRequest) {
  try {
    const session = await auth();
    if (!session?.user?.token) {
      return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
    }

    // Get the form data from the request
    const formData = await request.formData();

    // Forward to backend
    const response = await fetch(
      `${env.NEXT_PUBLIC_API_URL}/api/import/students/preview`,
      {
        method: "POST",
        headers: {
          Authorization: `Bearer ${session.user.token}`,
        },
        body: formData,
      },
    );

    const data = (await response.json()) as Record<string, unknown>;

    if (!response.ok) {
      return NextResponse.json(data, { status: response.status });
    }

    return NextResponse.json(data);
  } catch (error) {
    console.error("Preview error:", error);
    return NextResponse.json(
      { error: "Internal server error" },
      { status: 500 },
    );
  }
}
