import { type NextRequest, NextResponse } from "next/server";
import { env } from "~/env";

export async function POST(request: NextRequest) {
  try {
    const body: unknown = await request.json();

    const response = await fetch(`${env.NEXT_PUBLIC_API_URL}/auth/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });

    // Check if the response has a body and is JSON
    let data: unknown;
    const contentType = response.headers.get("content-type");

    if (contentType?.includes("application/json")) {
      try {
        data = await response.json();
      } catch (jsonError) {
        console.error("Failed to parse JSON response:", jsonError);
        data = { message: await response.text() };
      }
    } else {
      // If not JSON, get the text response
      const text = await response.text();
      data = { message: text ?? "Request failed with no response" };
    }

    return NextResponse.json(data ?? { message: "Empty response" }, {
      status: response.status,
    });
  } catch (error) {
    console.error("Login route error:", error);
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
