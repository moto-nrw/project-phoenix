// Test route that uses the standard Next.js 15 route handler API
import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth } from "~/server/auth";

export async function GET(
  _request: NextRequest
) {
  try {
    const session = await auth();
    
    if (!session?.user?.token) {
      return NextResponse.json(
        { error: "Unauthorized" },
        { status: 401 }
      );
    }
    
    return NextResponse.json({ 
      success: true, 
      message: "Test route successful" 
    });
  } catch (_error) {
    return NextResponse.json(
      { error: "An error occurred" },
      { status: 500 }
    );
  }
}