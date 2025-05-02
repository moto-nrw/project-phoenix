import type { NextRequest } from 'next/server';
import { NextResponse } from 'next/server';
import { auth } from '~/server/auth';
import { env } from '~/env';

export async function GET(request: NextRequest) {
  // Get authentication session
  const session = await auth();
  
  if (!session?.user?.token) {
    return NextResponse.json(
      { error: 'Unauthorized: No valid session' },
      { status: 401 }
    );
  }
  
  // Parse query parameters
  const searchParams = request.nextUrl.searchParams;
  const search = searchParams.get('search');
  const inHouse = searchParams.get('in_house');
  const groupId = searchParams.get('group_id');
  const wcParam = searchParams.get('wc');
  const schoolYard = searchParams.get('school_yard');
  
  // Build backend API URL with parameters
  const url = new URL(`${env.NEXT_PUBLIC_API_URL}/students`);
  if (search) url.searchParams.append('search', search);
  if (inHouse) url.searchParams.append('in_house', inHouse);
  if (groupId) url.searchParams.append('group_id', groupId);
  if (wcParam) url.searchParams.append('wc', wcParam);
  if (schoolYard) url.searchParams.append('school_yard', schoolYard);
  
  try {
    // Forward the request to the backend with token
    // Check if user has proper roles
    if (!session.user.roles || session.user.roles.length === 0) {
      console.warn('User has no roles for API request');
    }
    
    console.log('Making API request with roles:', session.user.roles);
    
    const backendResponse = await fetch(url.toString(), {
      headers: {
        'Authorization': `Bearer ${session.user.token}`,
        'Content-Type': 'application/json',
      },
    });
    
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
    console.error('Error fetching students:', error);
    return NextResponse.json(
      { error: 'Internal Server Error' },
      { status: 500 }
    );
  }
}

export async function POST(request: NextRequest) {
  // Get authentication session
  const session = await auth();
  
  if (!session?.user?.token) {
    return NextResponse.json(
      { error: 'Unauthorized: No valid session' },
      { status: 401 }
    );
  }
  
  try {
    // Get the request body
    const studentData = await request.json();
    
    // Use the new combined endpoint that creates both custom user and student in one request
    const url = new URL(`${env.NEXT_PUBLIC_API_URL}/students/with-user`);
    const backendResponse = await fetch(url.toString(), {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${session.user.token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(studentData),
    });
    
    if (!backendResponse.ok) {
      const errorText = await backendResponse.text();
      console.error(`Backend API error: ${backendResponse.status}`, errorText);
      
      // Try to parse error for better error messages
      try {
        const errorJson = JSON.parse(errorText) as { error?: string };
        return NextResponse.json(
          { error: errorJson.error ?? `Error creating student: ${backendResponse.status}` },
          { status: backendResponse.status }
        );
      } catch {
        // If parsing fails, use status code
        return NextResponse.json(
          { error: `Error creating student: ${backendResponse.status}` },
          { status: backendResponse.status }
        );
      }
    }
    
    const data: unknown = await backendResponse.json();
    return NextResponse.json(data);
  } catch (error: unknown) {
    console.error('Error creating student:', error);
    return NextResponse.json(
      { error: 'Internal Server Error' },
      { status: 500 }
    );
  }
}