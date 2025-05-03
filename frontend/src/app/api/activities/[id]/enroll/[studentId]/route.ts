import { NextRequest, NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { env } from "~/env";

interface RouteContext {
  params: {
    id: string;
    studentId: string;
  };
}

/**
 * POST handler for enrolling a student in an activity
 */
export async function POST(
  request: NextRequest,
  context: RouteContext
) {
  const { id, studentId } = context.params;
  const session = await auth();
  
  if (!session?.user?.token) {
    return NextResponse.json(
      { error: "Unauthorized" },
      { status: 401 }
    );
  }
  
  try {
    const apiUrl = `${env.NEXT_PUBLIC_API_URL}/activities/${id}/enroll/${studentId}`;
    const response = await fetch(apiUrl, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${session.user.token}`,
        'Content-Type': 'application/json',
      },
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
    console.error(`Error enrolling student ${studentId} in activity ${id}:`, error);
    return NextResponse.json(
      { error: 'Failed to enroll student' },
      { status: 500 }
    );
  }
}

/**
 * DELETE handler for unenrolling a student from an activity
 */
export async function DELETE(
  request: NextRequest,
  context: RouteContext
) {
  const { id, studentId } = context.params;
  const session = await auth();
  
  if (!session?.user?.token) {
    return NextResponse.json(
      { error: "Unauthorized" },
      { status: 401 }
    );
  }
  
  try {
    const apiUrl = `${env.NEXT_PUBLIC_API_URL}/activities/${id}/enroll/${studentId}`;
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
    console.error(`Error unenrolling student ${studentId} from activity ${id}:`, error);
    return NextResponse.json(
      { error: 'Failed to unenroll student' },
      { status: 500 }
    );
  }
}