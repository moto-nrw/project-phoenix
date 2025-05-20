// app/api/activities/test/enrollment/route.ts
import { NextResponse } from "next/server";
import { auth } from "../../../../../server/auth";
import { enrollStudent } from "~/lib/activity-api";

/**
 * A test endpoint to verify the enrollment functionality
 * This will call the enrollStudent function and return the result
 * GET /api/activities/test/enrollment?activityId=1&studentId=2
 */
export async function GET(request: Request) {
  try {
    const session = await auth();
    
    if (!session?.user?.token) {
      return NextResponse.json(
        { error: "Unauthorized" },
        { status: 401 }
      );
    }
    
    // Get query parameters
    const url = new URL(request.url);
    const activityId = url.searchParams.get('activityId');
    const studentId = url.searchParams.get('studentId');
    
    if (!activityId || !studentId) {
      return NextResponse.json(
        { error: "Missing activityId or studentId parameters" },
        { status: 400 }
      );
    }
    
    // Test the enrollStudent function
    try {
      const result = await enrollStudent(activityId, { studentId });
      
      return NextResponse.json({
        success: true,
        message: "Test successful",
        result,
      });
    } catch (error) {
      if (error instanceof Error) {
        return NextResponse.json({
          success: false,
          message: "Test failed",
          error: error.message,
        }, { status: 500 });
      }
      
      return NextResponse.json({
        success: false,
        message: "Test failed with unknown error",
      }, { status: 500 });
    }
  } catch (error) {
    console.error("Error in test endpoint:", error);
    return NextResponse.json(
      { error: "Internal server error" },
      { status: 500 }
    );
  }
}