import { NextRequest, NextResponse } from 'next/server';
import { auth } from '~/server/auth';
import { env } from '~/env';

const API_URL = env.NEXT_PUBLIC_API_URL;

export async function POST(
  request: NextRequest,
  { params }: { params: { id: string; studentId: string } }
) {
  const session = await auth();
  
  if (!session?.user?.token) {
    return NextResponse.json(
      { error: 'Unauthorized: No valid session' },
      { status: 401 }
    );
  }

  const { id, studentId } = await params;

  try {
    const response = await fetch(`${API_URL}/activities/${id}/enroll/${studentId}`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${session.user.token}`,
        'Content-Type': 'application/json',
      },
    });

    if (!response.ok) {
      const errorText = await response.text();
      console.error(`API error: ${response.status}`, errorText);
      
      // Try to parse error for better error messages
      try {
        const errorJson = JSON.parse(errorText);
        return NextResponse.json(
          { error: errorJson.error || `Error enrolling student: ${response.status}` },
          { status: response.status }
        );
      } catch (e) {
        // If parsing fails, use status code
        return NextResponse.json(
          { error: `Error enrolling student: ${response.status}` },
          { status: response.status }
        );
      }
    }

    const data = await response.json();
    return NextResponse.json(data);
  } catch (error) {
    console.error(`Error enrolling student ${studentId} in activity ${id}:`, error);
    return NextResponse.json(
      { error: 'Internal Server Error' },
      { status: 500 }
    );
  }
}

export async function DELETE(
  request: NextRequest,
  { params }: { params: { id: string; studentId: string } }
) {
  const session = await auth();
  
  if (!session?.user?.token) {
    return NextResponse.json(
      { error: 'Unauthorized: No valid session' },
      { status: 401 }
    );
  }

  const { id, studentId } = await params;

  try {
    const response = await fetch(`${API_URL}/activities/${id}/enroll/${studentId}`, {
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
        { error: `Backend error: ${response.status}` },
        { status: response.status }
      );
    }

    return NextResponse.json({ success: true });
  } catch (error) {
    console.error(`Error unenrolling student ${studentId} from activity ${id}:`, error);
    return NextResponse.json(
      { error: 'Internal Server Error' },
      { status: 500 }
    );
  }
}