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
      `${env.NEXT_PUBLIC_API_URL}/groups/${groupId}`,
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
      
      // Try to parse the error text as JSON for a more detailed error message
      try {
        const errorJson = JSON.parse(errorText) as { error?: string };
        // If the backend returned a specific error message, return that
        if (errorJson.error) {
          return NextResponse.json(
            { error: errorJson.error },
            { status: backendResponse.status }
          );
        }
      } catch {
        // If parsing fails, continue with the default error handling
      }
      
      return NextResponse.json(
        { error: `Backend error: ${backendResponse.status}` },
        { status: backendResponse.status }
      );
    }
    
    const data: unknown = await backendResponse.json();
    return NextResponse.json(data);
  } catch (error: unknown) {
    console.error(`Error fetching group ${groupId}:`, error);
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
    const requestBody = await request.json();
    
    // Forward the request to the backend with token
    const backendResponse = await fetch(
      `${env.NEXT_PUBLIC_API_URL}/groups/${groupId}`,
      {
        method: 'PUT',
        headers: {
          'Authorization': `Bearer ${session.user.token}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(requestBody),
      }
    );
    
    if (!backendResponse.ok) {
      const errorText = await backendResponse.text();
      console.error(`Backend API error: ${backendResponse.status}`, errorText);
      
      // Try to parse the error text as JSON for a more detailed error message
      try {
        const errorJson = JSON.parse(errorText) as { error?: string };
        // If the backend returned a specific error message, return that
        if (errorJson.error) {
          return NextResponse.json(
            { error: errorJson.error },
            { status: backendResponse.status }
          );
        }
      } catch {
        // If parsing fails, continue with the default error handling
      }
      
      return NextResponse.json(
        { error: `Backend error: ${backendResponse.status}` },
        { status: backendResponse.status }
      );
    }
    
    const data: unknown = await backendResponse.json();
    return NextResponse.json(data);
  } catch (error: unknown) {
    console.error(`Error updating group ${groupId}:`, error);
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
      `${env.NEXT_PUBLIC_API_URL}/groups/${groupId}`,
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
      
      // Try to parse the error text as JSON for a more detailed error message
      try {
        const errorJson = JSON.parse(errorText) as { error?: string };
        // If the backend returned a specific error message, return that
        if (errorJson.error) {
          return NextResponse.json(
            { error: errorJson.error },
            { status: backendResponse.status }
          );
        }
      } catch {
        // If parsing fails, continue with the default error handling
      }
      
      return NextResponse.json(
        { error: `Backend error: ${backendResponse.status}` },
        { status: backendResponse.status }
      );
    }
    
    return new NextResponse(null, { status: 204 });
  } catch (error: unknown) {
    console.error(`Error deleting group ${groupId}:`, error);
    return NextResponse.json(
      { error: 'Internal Server Error' },
      { status: 500 }
    );
  }
}