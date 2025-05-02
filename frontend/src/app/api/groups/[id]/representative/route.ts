import type { NextRequest } from 'next/server';
import { NextResponse } from 'next/server';
import { auth } from '~/server/auth';
import { env } from '~/env';

export async function GET(
  request: NextRequest,
  { params }: { params: { id: string } }
) {
  // Get authentication session
  const session = await auth();
  
  if (!session?.user?.token) {
    return NextResponse.json(
      { error: 'Unauthorized: No valid session' },
      { status: 401 }
    );
  }
  
  // Make sure params is fully resolved
  const resolvedParams = params instanceof Promise ? await params : params;
  const groupId: string = resolvedParams.id;
  
  try {
    // Check if user has proper roles
    if (!session.user.roles || session.user.roles.length === 0) {
      console.warn('User has no roles for API request');
    }
    
    console.log('Making API request with roles:', session.user.roles);
    
    // Forward the request to the backend with token
    const backendResponse = await fetch(
      `${env.NEXT_PUBLIC_API_URL}/groups/${groupId}/representative`,
      {
        headers: {
          'Authorization': `Bearer ${session.user.token}`,
          'Content-Type': 'application/json',
        },
      }
    );
    
    if (!backendResponse.ok) {
      const errorText = await backendResponse.text();
      console.error(`Backend API error: ${backendResponse.status}`, errorText);
      return NextResponse.json(
        { error: `Backend error: ${backendResponse.status}` },
        { status: backendResponse.status }
      );
    }
    
    const data: unknown = await backendResponse.json();
    return NextResponse.json(data);
  } catch (error: unknown) {
    console.error(`Error fetching representative for group ${groupId}:`, error);
    return NextResponse.json(
      { error: 'Internal Server Error' },
      { status: 500 }
    );
  }
}

export async function PUT(
  request: NextRequest,
  { params }: { params: { id: string } }
) {
  // Get authentication session
  const session = await auth();
  
  if (!session?.user?.token) {
    return NextResponse.json(
      { error: 'Unauthorized: No valid session' },
      { status: 401 }
    );
  }
  
  // Make sure params is fully resolved
  const resolvedParams = params instanceof Promise ? await params : params;
  const groupId: string = resolvedParams.id;
  
  try {
    // Parse request body
    const { studentId } = await request.json() as { studentId: string };
    
    if (!studentId) {
      return NextResponse.json(
        { error: 'Invalid request: studentId is required' },
        { status: 400 }
      );
    }
    
    // Forward the request to the backend with token
    const backendResponse = await fetch(
      `${env.NEXT_PUBLIC_API_URL}/groups/${groupId}/representative`,
      {
        method: 'PUT',
        headers: {
          'Authorization': `Bearer ${session.user.token}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ studentId }),
      }
    );
    
    if (!backendResponse.ok) {
      const errorText = await backendResponse.text();
      console.error(`Backend API error: ${backendResponse.status}`, errorText);
      
      // Try to parse error for better error messages
      try {
        const errorJson = JSON.parse(errorText) as { error?: string };
        return NextResponse.json(
          { error: errorJson.error ?? `Error setting group representative: ${backendResponse.status}` },
          { status: backendResponse.status }
        );
      } catch {
        // If parsing fails, use status code
        return NextResponse.json(
          { error: `Error setting group representative: ${backendResponse.status}` },
          { status: backendResponse.status }
        );
      }
    }
    
    const data: unknown = await backendResponse.json();
    return NextResponse.json(data);
  } catch (error: unknown) {
    console.error(`Error setting representative for group ${groupId}:`, error);
    return NextResponse.json(
      { error: 'Internal Server Error' },
      { status: 500 }
    );
  }
}

export async function DELETE(
  request: NextRequest,
  { params }: { params: { id: string } }
) {
  // Get authentication session
  const session = await auth();
  
  if (!session?.user?.token) {
    return NextResponse.json(
      { error: 'Unauthorized: No valid session' },
      { status: 401 }
    );
  }
  
  // Make sure params is fully resolved
  const resolvedParams = params instanceof Promise ? await params : params;
  const groupId: string = resolvedParams.id;
  
  try {
    // Forward the request to the backend with token
    const backendResponse = await fetch(
      `${env.NEXT_PUBLIC_API_URL}/groups/${groupId}/representative`,
      {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${session.user.token}`,
          'Content-Type': 'application/json',
        },
      }
    );
    
    if (!backendResponse.ok) {
      const errorText = await backendResponse.text();
      console.error(`Backend API error: ${backendResponse.status}`, errorText);
      return NextResponse.json(
        { error: `Backend error: ${backendResponse.status}` },
        { status: backendResponse.status }
      );
    }
    
    return new NextResponse(null, { status: 204 });
  } catch (error: unknown) {
    console.error(`Error removing representative from group ${groupId}:`, error);
    return NextResponse.json(
      { error: 'Internal Server Error' },
      { status: 500 }
    );
  }
}