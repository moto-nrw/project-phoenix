import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { env } from "~/env";

interface RouteContext {
  params: {
    id: string;
  };
}

/**
 * GET handler for fetching time slots for an activity
 */
export async function GET(request: NextRequest, context: RouteContext) {
  const { id } = context.params;
  const session = await auth();

  if (!session?.user?.token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  try {
    const apiUrl = `${env.NEXT_PUBLIC_API_URL}/activities/${id}/times`;
    const response = await fetch(apiUrl, {
      headers: {
        Authorization: `Bearer ${session.user.token}`,
        "Content-Type": "application/json",
      },
    });

    if (!response.ok) {
      const errorText = await response.text();
      console.error(`API error: ${response.status}`, errorText);

      return NextResponse.json(
        { error: `Error from API: ${response.statusText}` },
        { status: response.status },
      );
    }

    const data = (await response.json()) as Record<string, unknown>;
    return NextResponse.json(data);
  } catch (error) {
    console.error(`Error fetching time slots for activity ${id}:`, error);
    return NextResponse.json(
      { error: "Failed to fetch time slots" },
      { status: 500 },
    );
  }
}

/**
 * POST handler for adding a time slot to an activity
 */
export async function POST(request: NextRequest, context: RouteContext) {
  const { id } = context.params;
  const session = await auth();

  if (!session?.user?.token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  try {
    const body = (await request.json()) as {
      weekday?: string;
      timespan_id?: string;
      [key: string]: unknown;
    };

    // Basic validation
    if (!body.weekday) {
      return NextResponse.json(
        { error: "Weekday is required" },
        { status: 400 },
      );
    }

    if (!body.timespan_id) {
      return NextResponse.json(
        { error: "Timespan ID is required" },
        { status: 400 },
      );
    }

    const apiUrl = `${env.NEXT_PUBLIC_API_URL}/activities/${id}/times`;
    const response = await fetch(apiUrl, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${session.user.token}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify(body),
    });

    if (!response.ok) {
      const errorData = await response.text();
      console.error(`API error: ${response.status}`, errorData);

      let errorMessage = `Error from API: ${response.statusText}`;
      try {
        const parsedError = JSON.parse(errorData) as { error?: string };
        if (parsedError.error) {
          errorMessage = parsedError.error;
        }
      } catch {
        // Use default error message
      }

      return NextResponse.json(
        { error: errorMessage },
        { status: response.status },
      );
    }

    const data = (await response.json()) as Record<string, unknown>;
    return NextResponse.json(data, { status: 201 });
  } catch (error) {
    console.error(`Error adding time slot to activity ${id}:`, error);
    return NextResponse.json(
      { error: "Failed to add time slot" },
      { status: 500 },
    );
  }
}
