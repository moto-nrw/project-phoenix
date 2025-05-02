import type { NextRequest } from 'next/server';
import { NextResponse } from 'next/server';
import { auth } from '~/server/auth';
import { env } from '~/env';

const API_URL = env.NEXT_PUBLIC_API_URL;

export async function POST(request: NextRequest) {
  const session = await auth();
  
  if (!session?.user?.token) {
    return NextResponse.json(
      { error: 'Unauthorized: No valid session' },
      { status: 401 }
    );
  }

  try {
    const body: unknown = await request.json();
    
    const response = await fetch(`${API_URL}/activities/timespans`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${session.user.token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(body),
    });

    if (!response.ok) {
      const errorText = await response.text();
      console.error(`API error: ${response.status}`, errorText);
      
      // Try to parse error for better error messages
      try {
        const errorJson = JSON.parse(errorText) as { error?: string };
        return NextResponse.json(
          { error: errorJson.error ?? `Error creating timespan: ${response.status}` },
          { status: response.status }
        );
      } catch {
        // If parsing fails, use status code
        return NextResponse.json(
          { error: `Error creating timespan: ${response.status}` },
          { status: response.status }
        );
      }
    }

    const data: unknown = await response.json();
    return NextResponse.json(data);
  } catch (error: unknown) {
    console.error('Error creating timespan:', error);
    return NextResponse.json(
      { error: 'Internal Server Error' },
      { status: 500 }
    );
  }
}