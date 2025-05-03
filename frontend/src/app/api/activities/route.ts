import { NextRequest, NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { env } from "~/env";

/**
 * GET handler for fetching activities
 */
export async function GET(request: NextRequest) {
  const session = await auth();
  
  if (!session?.user?.token) {
    return NextResponse.json(
      { error: "Unauthorized" },
      { status: 401 }
    );
  }

  // Get query parameters from the request URL
  const url = new URL(request.url);
  const searchParams = url.searchParams;
  
  try {
    const apiUrl = new URL('/activities', env.NEXT_PUBLIC_API_URL);
    
    // Forward any query parameters
    searchParams.forEach((value, key) => {
      apiUrl.searchParams.append(key, value);
    });
    
    const response = await fetch(apiUrl.toString(), {
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
    console.error('Error fetching activities:', error);
    return NextResponse.json(
      { error: 'Failed to fetch activities' },
      { status: 500 }
    );
  }
}

/**
 * POST handler for creating activities
 */
export async function POST(request: NextRequest) {
  const session = await auth();
  
  if (!session?.user?.token) {
    return NextResponse.json(
      { error: "Unauthorized" },
      { status: 401 }
    );
  }
  
  try {
    const body = await request.json();
    
    const apiUrl = `${env.NEXT_PUBLIC_API_URL}/activities`;
    const response = await fetch(apiUrl, {
      method: 'POST',
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
    return NextResponse.json(data, { status: 201 });
  } catch (error) {
    console.error('Error creating activity:', error);
    return NextResponse.json(
      { error: 'Failed to create activity' },
      { status: 500 }
    );
  }
}