import { NextRequest, NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { env } from "~/env";

/**
 * GET handler for fetching activities a student can enroll in
 */
export async function GET(request: NextRequest) {
  const session = await auth();
  
  if (!session?.user?.token) {
    return NextResponse.json(
      { error: "Unauthorized" },
      { status: 401 }
    );
  }
  
  // Get student_id from query parameters
  const url = new URL(request.url);
  const studentId = url.searchParams.get('student_id');
  
  if (!studentId) {
    return NextResponse.json(
      { error: "student_id parameter is required" },
      { status: 400 }
    );
  }
  
  try {
    const apiUrl = `${env.NEXT_PUBLIC_API_URL}/activities/student/available?student_id=${studentId}`;
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
    console.error(`Error fetching available activities for student ${studentId}:`, error);
    return NextResponse.json(
      { error: 'Failed to fetch available activities' },
      { status: 500 }
    );
  }
}