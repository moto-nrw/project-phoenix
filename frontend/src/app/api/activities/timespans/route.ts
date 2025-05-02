import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { env } from "~/env";

interface TimespanRequest {
  start_time: string;
  end_time: string;
  [key: string]: unknown;
}

interface ErrorResponse {
  error: string;
  [key: string]: unknown;
}

/**
 * POST handler for creating a new timespan
 */
export async function POST(
  request: NextRequest
) {
  const session = await auth();
  
  if (!session?.user?.token) {
    return NextResponse.json(
      { error: "Unauthorized" },
      { status: 401 }
    );
  }
  
  try {
    const body = await request.json() as TimespanRequest;
    
    // Basic validation
    if (!body.start_time) {
      return NextResponse.json(
        { error: "Start time is required" },
        { status: 400 }
      );
    }
    
    if (!body.end_time) {
      return NextResponse.json(
        { error: "End time is required" },
        { status: 400 }
      );
    }
    
    const apiUrl = `${env.NEXT_PUBLIC_API_URL}/timespans`;
    console.log('Creating timespan at:', apiUrl);
    console.log('With body:', JSON.stringify(body));
    
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
        const parsedError = JSON.parse(errorData) as ErrorResponse;
        if (parsedError.error) {
          errorMessage = parsedError.error;
        }
      } catch {
        // Use default error message
      }
      
      return NextResponse.json(
        { error: errorMessage },
        { status: response.status }
      );
    }
    
    const data = await response.json() as unknown;
    console.log('Timespan created successfully:', JSON.stringify(data));
    return NextResponse.json(data, { status: 201 });
  } catch (error) {
    console.error(`Error creating timespan:`, error);
    return NextResponse.json(
      { error: 'Failed to create timespan' },
      { status: 500 }
    );
  }
}