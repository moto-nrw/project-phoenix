import { NextRequest, NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { env } from "~/env";

interface RouteContext {
  params: {
    id: string;
  };
}

/**
 * GET handler for fetching a specific activity by ID
 */
export async function GET(
  request: NextRequest,
  context: RouteContext
) {
  const resolvedParams = context.params instanceof Promise ? await context.params : context.params;
  const id = resolvedParams.id;
  const session = await auth();
  
  if (!session?.user?.token) {
    return NextResponse.json(
      { error: "Unauthorized" },
      { status: 401 }
    );
  }
  
  try {
    const apiUrl = `${env.NEXT_PUBLIC_API_URL}/activities/${id}`;
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
    console.error(`Error fetching activity ${id}:`, error);
    return NextResponse.json(
      { error: 'Failed to fetch activity' },
      { status: 500 }
    );
  }
}

/**
 * PUT handler for updating an activity
 */
export async function PUT(
  request: NextRequest,
  context: RouteContext
) {
  const resolvedParams = context.params instanceof Promise ? await context.params : context.params;
  const id = resolvedParams.id;
  const session = await auth();
  
  if (!session?.user?.token) {
    return NextResponse.json(
      { error: "Unauthorized" },
      { status: 401 }
    );
  }
  
  try {
    const body = await request.json();
    
    const apiUrl = `${env.NEXT_PUBLIC_API_URL}/activities/${id}`;
    const response = await fetch(apiUrl, {
      method: 'PUT',
      headers: {
        'Authorization': `Bearer ${session.user.token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(body),
    });
    
    if (!response.ok) {
      const errorData = await response.text();
      console.error(`API error: ${response.status}`, errorData);
      
      let errorMessage = `Error from API: ${response.statusText}`;
      try {
        const parsedError = JSON.parse(errorData);
        if (parsedError.error) {
          errorMessage = parsedError.error;
        }
      } catch (e) {
        // Use default error message
      }
      
      return NextResponse.json(
        { error: errorMessage },
        { status: response.status }
      );
    }
    
    const data = await response.json();
    return NextResponse.json(data);
  } catch (error) {
    console.error(`Error updating activity ${id}:`, error);
    return NextResponse.json(
      { error: 'Failed to update activity' },
      { status: 500 }
    );
  }
}

/**
 * DELETE handler for deleting an activity
 */
export async function DELETE(
  request: NextRequest,
  context: RouteContext
) {
  const resolvedParams = context.params instanceof Promise ? await context.params : context.params;
  const id = resolvedParams.id;
  const session = await auth();
  
  if (!session?.user?.token) {
    return NextResponse.json(
      { error: "Unauthorized" },
      { status: 401 }
    );
  }
  
  try {
    const apiUrl = `${env.NEXT_PUBLIC_API_URL}/activities/${id}`;
    const response = await fetch(apiUrl, {
      method: 'DELETE',
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
    
    return new NextResponse(null, { status: 204 });
  } catch (error) {
    console.error(`Error deleting activity ${id}:`, error);
    return NextResponse.json(
      { error: 'Failed to delete activity' },
      { status: 500 }
    );
  }
}