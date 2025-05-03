import { NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { env } from "~/env";

export async function GET() {
  const session = await auth();
  
  // Ensure user is authenticated
  if (!session?.user?.token) {
    return NextResponse.json({ error: "Unauthorized: No valid session" }, { status: 401 });
  }
  
  const apiUrl = `${env.NEXT_PUBLIC_API_URL}/rooms/by-category`;
  
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
    console.error("Error fetching rooms by category:", error);
    return NextResponse.json(
      { error: "Failed to fetch rooms by category" },
      { status: 500 }
    );
  }
}