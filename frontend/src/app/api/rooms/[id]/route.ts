import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { env } from "~/env";

export async function GET(
  request: NextRequest,
  { params }: { params: { id: string } }
) {
  const session = await auth();
  
  // Ensure user is authenticated
  if (!session?.user?.token) {
    return NextResponse.json({ error: "Unauthorized: No valid session" }, { status: 401 });
  }
  
  const { id } = params;
  const apiUrl = `${env.NEXT_PUBLIC_API_URL}/rooms/${id}`;
  
  try {
    // Forward the request to the backend API
    const response = await fetch(apiUrl, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${session.user.token}`,
      },
    });
    
    if (!response.ok) {
      return NextResponse.json(
        { error: `API error: ${response.status}` },
        { status: response.status }
      );
    }
    
    const data: unknown = await response.json();
    return NextResponse.json(data);
  } catch (error) {
    console.error(`Error fetching room ${id}:`, error);
    return NextResponse.json(
      { error: "Failed to fetch room" },
      { status: 500 }
    );
  }
}

export async function PUT(
  request: NextRequest,
  { params }: { params: { id: string } }
) {
  const session = await auth();
  
  // Ensure user is authenticated
  if (!session?.user?.token) {
    return NextResponse.json({ error: "Unauthorized: No valid session" }, { status: 401 });
  }
  
  const { id } = params;
  const apiUrl = `${env.NEXT_PUBLIC_API_URL}/rooms/${id}`;
  
  try {
    // Get the request body
    const roomData: unknown = await request.json();

    // Forward the request to the backend API
    const response = await fetch(apiUrl, {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${session.user.token}`,
      },
      body: JSON.stringify(roomData),
    });
    
    if (!response.ok) {
      const errorText = await response.text();
      console.error(`API error: ${response.status}`, errorText);

      // Try to parse error for better error messages
      try {
        const errorJson = JSON.parse(errorText) as { error?: string };
        return NextResponse.json(
          {
            error: errorJson.error ?? `Error updating room: ${response.status}`,
          },
          { status: response.status },
        );
      } catch {
        // If parsing fails, use status code
        return NextResponse.json(
          { error: `Error updating room: ${response.status}` },
          { status: response.status },
        );
      }
    }
    
    const data: unknown = await response.json();
    return NextResponse.json(data);
  } catch (error) {
    console.error(`Error updating room ${id}:`, error);
    return NextResponse.json(
      { error: "Failed to update room" },
      { status: 500 }
    );
  }
}

export async function DELETE(
  request: NextRequest,
  { params }: { params: { id: string } }
) {
  const session = await auth();
  
  // Ensure user is authenticated
  if (!session?.user?.token) {
    return NextResponse.json({ error: "Unauthorized: No valid session" }, { status: 401 });
  }
  
  const { id } = params;
  const apiUrl = `${env.NEXT_PUBLIC_API_URL}/rooms/${id}`;
  
  try {
    // Forward the request to the backend API
    const response = await fetch(apiUrl, {
      method: "DELETE",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${session.user.token}`,
      },
    });
    
    if (!response.ok) {
      const errorText = await response.text();
      console.error(`API error: ${response.status}`, errorText);
      return NextResponse.json(
        { error: `API error: ${response.status}` },
        { status: response.status }
      );
    }
    
    return new NextResponse(null, { status: 204 });
  } catch (error) {
    console.error(`Error deleting room ${id}:`, error);
    return NextResponse.json(
      { error: "Failed to delete room" },
      { status: 500 }
    );
  }
}