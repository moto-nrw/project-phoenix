// app/api/activities/test/route.ts
import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { activityService } from "~/lib/activity-service";

/**
 * Simple test endpoint to verify our activity API implementation
 */
export async function GET(request: NextRequest) {
  try {
    // Test fetching activities
    const activities = await activityService.getActivities();
    
    // If we have activities, test getting the first one
    let activityDetail = null;
    let enrolledStudents = [];
    
    if (activities.length > 0) {
      const firstActivity = activities[0];
      activityDetail = await activityService.getActivity(firstActivity.id);
      
      // Test fetching enrolled students
      enrolledStudents = await activityService.getEnrolledStudents(firstActivity.id);
    }
    
    // Return success with test results
    return NextResponse.json({ 
      success: true,
      test_results: {
        activities_count: activities.length,
        has_activity_detail: !!activityDetail,
        enrolled_students_count: enrolledStudents.length
      },
      activities: activities.slice(0, 3), // Show just first 3 for brevity
      activity_detail: activityDetail,
      enrolled_students: enrolledStudents.slice(0, 5) // Show just first 5 for brevity
    });
  } catch (error) {
    console.error("Error in test endpoint:", error);
    
    return NextResponse.json({ 
      success: false,
      error: error instanceof Error ? error.message : String(error)
    }, { status: 500 });
  }
}