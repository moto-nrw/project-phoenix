import { NextRequest, NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { env } from "~/env";

/**
 * GET handler for fetching all supervisors
 * This is a proxy to the backend API
 */
export async function GET(req: NextRequest) {
  try {
    // Get session with auth helper
    const session = await auth();
    
    if (!session?.user?.token) {
      return NextResponse.json(
        { error: "Unauthorized" },
        { status: 401 }
      );
    }

    // Forward request to the backend API
    const response = await fetch(`${env.NEXT_PUBLIC_API_URL}/users/public/supervisors`, {
      method: "GET",
      headers: {
        "Authorization": `Bearer ${session.user.token}`,
        "Content-Type": "application/json",
      },
    });

    // If backend returns an error, forward it
    if (!response.ok) {
      const errorText = await response.text();
      console.error(`Error fetching supervisors: ${response.status}`, errorText);
      
      return NextResponse.json(
        { error: `Failed to fetch supervisors: ${response.statusText}` },
        { status: response.status }
      );
    }

    // Get data from backend
    const supervisors = await response.json();

    // Transform response to match frontend model if needed
    const formattedSupervisors = supervisors.map((supervisor: any) => ({
      id: supervisor.id.toString(),
      name: `${supervisor.first_name} ${supervisor.second_name}`.trim(),
      role: supervisor.role
    }));

    return NextResponse.json(formattedSupervisors);
  } catch (error) {
    console.error("Error in supervisors API route:", error);
    return NextResponse.json(
      { error: "Internal server error" },
      { status: 500 }
    );
  }
}