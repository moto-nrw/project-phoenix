import { NextRequest, NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { env } from "~/env";

interface RouteContext {
  params: {
    id: string;
  };
}

/**
 * GET handler for fetching activities a student is enrolled in
 */
export async function GET(
  request: NextRequest,
  context: RouteContext
) {
  const { id } = context.params;
  const session = await auth();
  
  if (!session?.user?.token) {
    return NextResponse.json(
      { error: "Unauthorized" },
      { status: 401 }
    );
  }
  
  try {
    const apiUrl = `${env.NEXT_PUBLIC_API_URL}/activities/student/${id}/ags`;
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
    
    const data = await response.json();
    return NextResponse.json(data);
  } catch (error) {
    console.error(`Error fetching activities for student ${id}:`, error);
    return NextResponse.json(
      { error: 'Failed to fetch student activities' },
      { status: 500 }
    );
  }
}