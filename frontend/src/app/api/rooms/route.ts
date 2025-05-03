import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { env } from "~/env";

export async function GET(request: NextRequest) {
  const session = await auth();
  
  // Ensure user is authenticated
  if (!session?.user?.token) {
    return NextResponse.json({ error: "Unauthorized: No valid session" }, { status: 401 });
  }
  
  // Get the query parameters
  const searchParams = request.nextUrl.searchParams;
  const search = searchParams.get("search");
  const building = searchParams.get("building");
  const floor = searchParams.get("floor");
  const category = searchParams.get("category");
  const occupied = searchParams.get("occupied");
  
  // Build the API URL with query parameters
  let apiUrl = `${env.NEXT_PUBLIC_API_URL}/rooms`;
  const queryParams = new URLSearchParams();
  
  if (search) queryParams.append("search", search);
  if (building) queryParams.append("building", building);
  if (floor) queryParams.append("floor", floor);
  if (category) queryParams.append("category", category);
  if (occupied) queryParams.append("occupied", occupied);
  
  const queryString = queryParams.toString();
  if (queryString) {
    apiUrl += `?${queryString}`;
  }
  
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
    console.error("Error fetching rooms:", error);
    return NextResponse.json(
      { error: "Failed to fetch rooms" },
      { status: 500 }
    );
  }
}

export async function POST(request: NextRequest) {
  // Get authentication session
  const session = await auth();

  if (!session?.user?.token) {
    return NextResponse.json(
      { error: "Unauthorized: No valid session" },
      { status: 401 },
    );
  }

  try {
    // Get the request body
    const roomData: unknown = await request.json();

    const url = `${env.NEXT_PUBLIC_API_URL}/rooms`;
    const backendResponse = await fetch(url, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${session.user.token}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify(roomData),
    });

    if (!backendResponse.ok) {
      const errorText = await backendResponse.text();
      console.error(`Backend API error: ${backendResponse.status}`, errorText);

      // Try to parse error for better error messages
      try {
        const errorJson = JSON.parse(errorText) as { error?: string };
        return NextResponse.json(
          {
            error:
              errorJson.error ??
              `Error creating room: ${backendResponse.status}`,
          },
          { status: backendResponse.status },
        );
      } catch {
        // If parsing fails, use status code
        return NextResponse.json(
          { error: `Error creating room: ${backendResponse.status}` },
          { status: backendResponse.status },
        );
      }
    }

    const data: unknown = await backendResponse.json();
    return NextResponse.json(data);
  } catch (error: unknown) {
    console.error("Error creating room:", error);
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}