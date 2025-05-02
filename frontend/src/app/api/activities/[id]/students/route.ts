import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { env } from "~/env";

interface RouteContext {
  params: {
    id: string;
  } | Promise<{
    id: string;
  }>;
}

/**
 * GET handler for fetching students enrolled in an activity
 */
export async function GET(
  request: NextRequest,
  context: RouteContext
) {
  const resolvedParams = await (context.params instanceof Promise ? context.params : Promise.resolve(context.params));
  const id = resolvedParams.id;
  const session = await auth();
  
  if (!session?.user?.token) {
    return NextResponse.json(
      { error: "Unauthorized" },
      { status: 401 }
    );
  }
  
  try {
    const apiUrl = `${env.NEXT_PUBLIC_API_URL}/activities/${id}/students`;
    const response = await fetch(apiUrl, {
      headers: {
        'Authorization': `Bearer ${session.user.token}`,
        'Content-Type': 'application/json',
      },
    });
    
    if (!response.ok) {
      const errorText = await response.text();
      console.error(`API error: ${response.status}`, errorText);
      
      return NextResponse.json(
        { error: `Error from API: ${response.statusText}` },
        { status: response.status }
      );
    }
    
    const data = await response.json() as Record<string, unknown>;
    return NextResponse.json(data);
  } catch (error) {
    console.error(`Error fetching students for activity ${id}:`, error);
    return NextResponse.json(
      { error: 'Failed to fetch enrolled students' },
      { status: 500 }
    );
  }
}